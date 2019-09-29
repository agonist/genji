package query

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"sync"

	"github.com/asdine/genji/database"
	"github.com/asdine/genji/record"
)

type Connector struct {
	driver driver.Driver
}

func NewConnector(db *database.DB) driver.Connector {
	return Connector{
		driver: newDriver(db),
	}
}

func (c Connector) Connect(ctx context.Context) (driver.Conn, error) {
	return c.driver.Open("")
}

func (c Connector) Driver() driver.Driver {
	return c.driver
}

type drivr struct {
	db *database.DB
}

func newDriver(db *database.DB) driver.Driver {
	return drivr{
		db: db,
	}
}

// Open returns a new connection to the database.
// The name is a string in a driver-specific format.
//
// Open may return a cached connection (one previously
// closed), but doing so is unnecessary; the sql package
// maintains a pool of idle connections for efficient re-use.
//
// The returned connection is only used by one goroutine at a
// time.
func (d drivr) Open(name string) (driver.Conn, error) {
	return conn{db: d.db}, nil
}

// Conn represents a connection to the Genji database.
// It implements the database/sql/driver.Conn interface.
type conn struct {
	db *database.DB
}

// Prepare returns a prepared statement, bound to this connection.
func (c conn) Prepare(q string) (driver.Stmt, error) {
	s, err := ParseStatement(q)
	if err != nil {
		return nil, err
	}

	return stmt{
		txo:  &TxOpener{DB: c.db},
		stmt: s,
	}, nil
}

// Close does nothing.
func (c conn) Close() error {
	return nil
}

// Begin starts and returns a new transaction.
func (c conn) Begin() (driver.Tx, error) {
	return c.db.Begin(true)
}

// BeginTx starts and returns a new transaction.
// It uses the ReadOnly option to determine whether to start a read-only or read/write transaction.
// If the Isolation option is non zero, an error is returned.
func (c conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if opts.Isolation != 0 {
		return nil, errors.New("isolation levels are not supported")
	}

	return c.db.Begin(!opts.ReadOnly)
}

// Stmt is a prepared statement. It is bound to a Conn and not
// used by multiple goroutines concurrently.
type stmt struct {
	txo  *TxOpener
	stmt Statement
}

// NumInput returns the number of placeholder parameters.
func (s stmt) NumInput() int { return -1 }

// Exec executes a query that doesn't return rows, such
// as an INSERT or UPDATE.
func (s stmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, errors.New("not implemented")
}

// CheckNamedValue has the same behaviour as driver.DefaultParamaterConverter, except that
// it allows record.Records to be passed as parameters.
// It implements the driver.NamedValueChecker interface.
func (s stmt) CheckNamedValue(nv *driver.NamedValue) error {
	if _, ok := nv.Value.(record.Record); ok {
		return nil
	}

	var err error
	nv.Value, err = driver.DefaultParameterConverter.ConvertValue(nv.Value)
	return err
}

// ExecContext executes a query that doesn't return rows, such
// as an INSERT or UPDATE.
func (s stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	res := s.stmt.Run(s.txo, args)

	return res, res.Err()
}

func (s stmt) Query(args []driver.Value) (driver.Rows, error) {
	return nil, errors.New("not implemented")
}

// QueryContext executes a query that may return rows, such as a
// SELECT.
func (s stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	res := s.stmt.Run(s.txo, args)

	if err := res.Err(); err != nil {
		return nil, err
	}

	rs := newRecordStream(res)
	slct, ok := s.stmt.(SelectStmt)
	if ok && len(slct.fieldSelectors) > 0 {
		rs.fields = make([]string, len(slct.fieldSelectors))
		for i := range slct.fieldSelectors {
			rs.fields[i] = slct.fieldSelectors[i].Name()
		}
	}
	return rs, nil
}

// Close does nothing.
func (s stmt) Close() error {
	return nil
}

type recordStream struct {
	res      Result
	cancelFn func()
	c        chan rec
	wg       sync.WaitGroup
	fields   []string
}

type rec struct {
	r   record.Record
	err error
}

func newRecordStream(res Result) *recordStream {
	ctx, cancel := context.WithCancel(context.Background())

	records := recordStream{
		res:      res,
		cancelFn: cancel,
		c:        make(chan rec),
	}
	records.wg.Add(1)

	go records.iterate(ctx)

	return &records
}

var errStop = errors.New("stop")

func (rs *recordStream) iterate(ctx context.Context) {
	defer rs.wg.Done()
	defer close(rs.c)

	select {
	case <-ctx.Done():
		return
	case <-rs.c:
	}

	err := rs.res.Iterate(func(r record.Record) error {
		select {
		case <-ctx.Done():
			return errStop
		case rs.c <- rec{
			r: r,
		}:

			select {
			case <-ctx.Done():
				return errStop
			case <-rs.c:
				return nil
			}
		}

	})

	if err == errStop || err == nil {
		return
	}
	if err != nil {
		rs.c <- rec{
			err: err,
		}
		return
	}
}

// Columns returns the fields selected by the SELECT statement.
// If the wildcard was used, it returns one column named "record".
func (rs *recordStream) Columns() []string {
	if len(rs.fields) > 0 {
		return rs.fields
	}

	return []string{"record"}
}

// Close closes the rows iterator.
func (rs *recordStream) Close() error {
	rs.cancelFn()
	return nil
}

// Next expects exactly one destination. This destination must implement record.Scanner
// otherwise an error is returned.
func (rs *recordStream) Next(dest []driver.Value) error {
	rs.c <- rec{}

	rec, ok := <-rs.c
	if !ok {
		return io.EOF
	}

	if rec.err != nil {
		return rec.err
	}

	if len(rs.fields) == 0 {
		dest[0] = rec.r
		return nil
	}

	for i := range rs.fields {
		f, err := rec.r.GetField(rs.fields[i])
		if err != nil {
			return err
		}

		dest[i], err = f.Decode()
		if err != nil {
			return err
		}
	}

	return nil
}
