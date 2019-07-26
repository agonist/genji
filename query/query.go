package query

import (
	"errors"
	"fmt"

	"github.com/asdine/genji"
	"github.com/asdine/genji/engine"
	"github.com/asdine/genji/field"
	"github.com/asdine/genji/record"
	"github.com/asdine/genji/table"
)

// Result of a query.
type Result struct {
	t   table.Reader
	err error
}

// Err returns a non nil error if an error occured during the query.
func (q Result) Err() error {
	return q.err
}

// Scan takes a table scanner and passes it the result table.
func (q Result) Scan(s table.Scanner) error {
	if q.err != nil {
		return q.err
	}

	return s.ScanTable(q.t)
}

// Table returns the table result.
func (q Result) Table() table.Reader {
	return q.t
}

// SelectStmt is a DSL that allows creating a full Select query.
// It is typically created using the Select function.
type SelectStmt struct {
	fieldSelectors []FieldSelector
	tableSelector  TableSelector
	whereExpr      Expr
	offsetExpr     Expr
	limitExpr      Expr
}

// Select creates a DSL equivalent to the SQL Select command.
// It takes a list of field selectors that indicate what fields must be selected from the targeted table.
// This package provides typed field selectors that can be used with the Select method.
func Select(selectors ...FieldSelector) SelectStmt {
	return SelectStmt{fieldSelectors: selectors}
}

// Run the Select query within tx.
// If Where was called, records will be filtered depending on the result of the
// given expression. If the Where expression implements the IndexMatcher interface,
// the MatchIndex method will be called instead of the Eval one.
func (q SelectStmt) Run(tx *genji.Tx) Result {
	if q.tableSelector == nil {
		return Result{err: errors.New("missing table selector")}
	}

	var offset int64 = -1
	var limit int64 = -1

	if q.offsetExpr != nil {
		s, err := q.offsetExpr.Eval(EvalContext{
			Tx: tx,
		})
		if err != nil {
			return Result{err: err}
		}
		if s.Type < field.Int {
			return Result{err: fmt.Errorf("offset expression must evaluate to a 64 bit integer, got %q", s.Type)}
		}
		offset, err = field.DecodeInt64(s.Data)
		if err != nil {
			return Result{err: err}
		}
	}

	if q.limitExpr != nil {
		s, err := q.limitExpr.Eval(EvalContext{
			Tx: tx,
		})
		if err != nil {
			return Result{err: err}
		}
		if s.Type < field.Int {
			return Result{err: fmt.Errorf("limit expression must evaluate to a 64 bit integer, got %q", s.Type)}
		}
		limit, err = field.DecodeInt64(s.Data)
		if err != nil {
			return Result{err: err}
		}
	}

	t, err := q.tableSelector.SelectTable(tx)
	if err != nil {
		return Result{err: err}
	}

	var b table.Browser

	if im, ok := q.whereExpr.(IndexMatcher); ok {
		tree, ok, err := im.MatchIndex(tx, q.tableSelector.Name())
		if err != nil && err != engine.ErrIndexNotFound {
			return Result{err: err}
		}

		if ok && err == nil {
			b.Reader = &indexResultTable{
				tree:  tree,
				table: t,
			}
		}
	}

	if b.Reader == nil {
		b.Reader, err = whereClause(tx, t, q.whereExpr, limit, offset)
		if err != nil {
			return Result{err: err}
		}
	} else {
		b = b.Offset(int(offset)).Limit(int(limit))
	}

	b = b.Map(func(recordID []byte, r record.Record) (record.Record, error) {
		var fb record.FieldBuffer

		for _, s := range q.fieldSelectors {
			f, err := s.SelectField(r)
			if err != nil {
				return nil, err
			}

			fb.Add(f)
		}

		return &fb, nil
	})

	if b.Err() != nil {
		return Result{err: b.Err()}
	}

	return Result{t: b.Reader}
}

// Where uses e to filter records if it evaluates to a falsy value.
func (q SelectStmt) Where(e Expr) SelectStmt {
	q.whereExpr = e
	return q
}

// From indicates which table to select from.
// Calling this method before Run is mandatory.
func (q SelectStmt) From(tableSelector TableSelector) SelectStmt {
	q.tableSelector = tableSelector
	return q
}

// Limit the number of records returned.
func (q SelectStmt) Limit(offset int) SelectStmt {
	q.limitExpr = Int64Value(int64(offset))
	return q
}

// LimitExpr takes an expression that will be evaluated to determine
// how many records the query must return.
// The result of the evaluation must be an integer.
func (q SelectStmt) LimitExpr(e Expr) SelectStmt {
	q.limitExpr = e
	return q
}

// Offset indicates the number of records to skip.
func (q SelectStmt) Offset(offset int) SelectStmt {
	q.offsetExpr = Int64Value(int64(offset))
	return q
}

// OffsetExpr takes an expression that will be evaluated to determine
// how many records the query must skip.
// The result of the evaluation must be a field.Int64.
func (q SelectStmt) OffsetExpr(e Expr) SelectStmt {
	q.offsetExpr = e
	return q
}

var errStop = errors.New("stop")

func whereClause(tx *genji.Tx, t table.Reader, e Expr, limit, offset int64) (table.Reader, error) {
	var skipped, count int64

	b := table.NewBrowser(t).Filter(func(_ []byte, r record.Record) (bool, error) {
		sc, err := e.Eval(EvalContext{Tx: tx, Record: r})
		if err != nil {
			return false, err
		}

		ok := sc.Truthy()
		if !ok {
			return false, nil
		}

		if skipped < offset {
			skipped++
			return false, nil
		}

		if limit >= 0 && count >= limit {
			return false, errStop
		}

		count++

		return true, nil
	})

	err := b.Err()
	if err != nil && err != errStop {
		return b.Reader, err
	}

	return b.Reader, nil
}

// DeleteStmt is a DSL that allows creating a full Delete query.
// It is typically created using the Delete function.
type DeleteStmt struct {
	tableSelector TableSelector
	whereExpr     Expr
}

// Delete creates a DSL equivalent to the SQL Delete command.
func Delete() DeleteStmt {
	return DeleteStmt{}
}

// From indicates which table to select from.
// Calling this method before Run is mandatory.
func (d DeleteStmt) From(tableSelector TableSelector) DeleteStmt {
	d.tableSelector = tableSelector
	return d
}

// Where uses e to filter records if it evaluates to a falsy value.
// Calling this method is optional.
func (d DeleteStmt) Where(e Expr) DeleteStmt {
	d.whereExpr = e
	return d
}

// Run the Delete query within tx.
// If Where was called, records will be filtered depending on the result of the
// given expression. If the Where expression implements the IndexMatcher interface,
// the MatchIndex method will be called instead of the Eval one.
func (d DeleteStmt) Run(tx *genji.Tx) error {
	if d.tableSelector == nil {
		return errors.New("missing table selector")
	}

	t, err := d.tableSelector.SelectTable(tx)
	if err != nil {
		return err
	}

	var b table.Browser

	if im, ok := d.whereExpr.(IndexMatcher); ok {
		tree, ok, err := im.MatchIndex(tx, d.tableSelector.Name())
		if err != nil && err != engine.ErrIndexNotFound {
			return err
		}

		if ok && err == nil {
			b.Reader = &indexResultTable{
				tree:  tree,
				table: t,
			}
		}
	}

	if b.Reader == nil {
		b.Reader, err = whereClause(tx, t, d.whereExpr, -1, -1)
		if err != nil {
			return err
		}
	}

	b = b.ForEach(func(recordID []byte, r record.Record) error {
		return t.Delete(recordID)
	})

	return b.Err()
}

// InsertStmt is a DSL that allows creating a full Insert query.
// It is typically created using the Insert function.
type InsertStmt struct {
	tableSelector TableSelector
	fieldNames    []string
	values        []Expr
}

// Insert creates a DSL equivalent to the SQL Insert command.
func Insert() InsertStmt {
	return InsertStmt{}
}

// Into indicates in which table to write the new records.
// Calling this method before Run is mandatory.
func (i InsertStmt) Into(tableSelector TableSelector) InsertStmt {
	i.tableSelector = tableSelector
	return i
}

// Fields to associate with values passed to the Values method.
func (i InsertStmt) Fields(fieldNames ...string) InsertStmt {
	i.fieldNames = append(i.fieldNames, fieldNames...)
	return i
}

// Values to associate with the record fields.
func (i InsertStmt) Values(values ...Expr) InsertStmt {
	i.values = append(i.values, values...)
	return i
}

// Run the Insert query within tx.
// For schemaless tables:
// - If the Fields method was called prior to the Run method, each value will be associated with one of the given field name, in order.
// - If the Fields method wasn't called, this will return an error
//
// For schemafull tables:
// - If the Fields method was called prior to the Run method, each value will be associated with one of the given field name, in order.
// Missing fields will be fields with their zero values.
// - If the Fields method wasn't called, this number of values must match the number of fields of the schema, and each value will be stored in
// each field of the schema, in order.
func (i InsertStmt) Run(tx *genji.Tx) Result {
	if i.tableSelector == nil {
		return Result{err: errors.New("missing table selector")}
	}

	if i.values == nil {
		return Result{err: errors.New("empty values")}
	}

	t, err := i.tableSelector.SelectTable(tx)
	if err != nil {
		return Result{err: err}
	}

	if len(i.fieldNames) == 0 {
		return i.runWithoutSelectedFields(tx, t)
	}

	schema, schemaful := t.Schema()
	if !schemaful {
		return i.runSchemalessWithSelectedFields(tx, t)
	}

	return i.runSchemafulWithSelectedFields(tx, t, &schema)
}

func (i InsertStmt) runWithoutSelectedFields(tx *genji.Tx, t *genji.Table) Result {
	schema, schemaful := t.Schema()

	if !schemaful {
		return Result{err: errors.New("fields must be selected for schemaless tables")}
	}

	var fb record.FieldBuffer

	if len(schema.Fields) != len(i.values) {
		return Result{err: fmt.Errorf("table %s has %d fields, got %d fields", i.tableSelector.Name(), len(schema.Fields), len(i.values))}
	}

	for idx, sf := range schema.Fields {
		sc, err := i.values[idx].Eval(EvalContext{
			Tx: tx,
		})
		if err != nil {
			return Result{err: err}
		}

		if sc.Type != sf.Type {
			return Result{err: fmt.Errorf("cannot assign value of type %q into field of type %q", sc.Type, sf.Type)}
		}

		fb.Add(field.Field{
			Name: sf.Name,
			Type: sf.Type,
			Data: sc.Data,
		})
	}

	recordID, err := t.Insert(&fb)
	if err != nil {
		return Result{err: err}
	}

	return Result{t: recordIDToTable(recordID)}
}

func recordIDToTable(recordID []byte) table.Table {
	var rb table.RecordBuffer
	rb.Insert(record.FieldBuffer([]field.Field{
		field.NewBytes("recordID", recordID),
	}))
	return &rb
}

func (i InsertStmt) runSchemalessWithSelectedFields(tx *genji.Tx, t *genji.Table) Result {
	var fb record.FieldBuffer

	if len(i.fieldNames) != len(i.values) {
		return Result{err: fmt.Errorf("%d values for %d fields", len(i.values), len(i.fieldNames))}
	}

	for idx, name := range i.fieldNames {
		sc, err := i.values[idx].Eval(EvalContext{
			Tx: tx,
		})
		if err != nil {
			return Result{err: err}
		}

		fb.Add(field.Field{
			Name: name,
			Type: sc.Type,
			Data: sc.Data,
		})
	}

	recordID, err := t.Insert(&fb)
	if err != nil {
		return Result{err: err}
	}

	return Result{t: recordIDToTable(recordID)}
}

func (i InsertStmt) runSchemafulWithSelectedFields(tx *genji.Tx, t *genji.Table, schema *record.Schema) Result {
	var fb record.FieldBuffer

	for _, sf := range schema.Fields {
		var found bool
		for idx, name := range i.fieldNames {
			if name != sf.Name {
				continue
			}

			sc, err := i.values[idx].Eval(EvalContext{
				Tx: tx,
			})
			if err != nil {
				return Result{err: err}
			}
			if sc.Type != sf.Type {
				return Result{err: fmt.Errorf("cannot assign value of type %q into field of type %q", sc.Type, sf.Type)}
			}
			fb.Add(field.Field{
				Name: name,
				Type: sc.Type,
				Data: sc.Data,
			})
			found = true
		}

		if !found {
			zv := field.ZeroValue(sf.Type)
			zv.Name = sf.Name
			fb.Add(zv)
		}
	}

	recordID, err := t.Insert(&fb)
	if err != nil {
		return Result{err: err}
	}

	return Result{t: recordIDToTable(recordID)}
}

// UpdateStmt is a DSL that allows creating a full Update query.
// It is typically created using the Update function.
type UpdateStmt struct {
	tableSelector TableSelector
	pairs         map[string]Expr
	whereExpr     Expr
}

// Update creates a DSL equivalent to the SQL Update command.
func Update(tableSelector TableSelector) UpdateStmt {
	return UpdateStmt{
		tableSelector: tableSelector,
		pairs:         make(map[string]Expr),
	}
}

// Set assignes the result of the evaluation of e into the field selected
// by f.
func (u UpdateStmt) Set(fieldName string, e Expr) UpdateStmt {
	u.pairs[fieldName] = e
	return u
}

// Where uses e to filter records if it evaluates to a falsy value.
// Calling this method is optional.
func (u UpdateStmt) Where(e Expr) UpdateStmt {
	u.whereExpr = e
	return u
}

// Run the Update query within tx.
// If Where was called, records will be filtered depending on the result of the
// given expression. If the Where expression implements the IndexMatcher interface,
// the MatchIndex method will be called instead of the Eval one.
func (u UpdateStmt) Run(tx *genji.Tx) error {
	if u.tableSelector == nil {
		return errors.New("missing table selector")
	}

	if len(u.pairs) == 0 {
		return errors.New("Set method not called")
	}

	t, err := u.tableSelector.SelectTable(tx)
	if err != nil {
		return err
	}

	var b table.Browser

	if im, ok := u.whereExpr.(IndexMatcher); ok {
		tree, ok, err := im.MatchIndex(tx, u.tableSelector.Name())
		if err != nil && err != engine.ErrIndexNotFound {
			return err
		}

		if ok && err == nil {
			b.Reader = &indexResultTable{
				tree:  tree,
				table: t,
			}
		}
	}

	if b.Reader == nil {
		b.Reader, err = whereClause(tx, t, u.whereExpr, -1, -1)
		if err != nil {
			return err
		}
	}

	schema, schemaful := t.Schema()

	b = b.ForEach(func(recordID []byte, r record.Record) error {
		var fb record.FieldBuffer
		err := fb.ScanRecord(r)
		if err != nil {
			return err
		}

		for fname, e := range u.pairs {
			f, err := fb.Field(fname)
			if err != nil {
				return err
			}

			s, err := e.Eval(EvalContext{
				Tx:     tx,
				Record: r,
			})
			if err != nil {
				return err
			}

			if schemaful {
				sf, err := schema.Field(fname)
				if err != nil {
					return err
				}
				if f.Type != sf.Type {
					return fmt.Errorf("cannot assign value of type %q into field of type %q", f.Type, sf.Type)
				}
			}

			f.Type = s.Type
			f.Data = s.Data
			err = fb.Replace(f.Name, f)
			if err != nil {
				return err
			}

			err = t.Replace(recordID, &fb)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return b.Err()
}
