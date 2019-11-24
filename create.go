package genji

import (
	"github.com/asdine/genji/internal/scanner"
	"github.com/asdine/genji/value"
)

// parseCreateStatement parses a create string and returns a Statement AST object.
// This function assumes the CREATE token has already been consumed.
func (p *parser) parseCreateStatement() (statement, error) {
	tok, pos, lit := p.ScanIgnoreWhitespace()
	switch tok {
	case scanner.TABLE:
		return p.parseCreateTableStatement()
	case scanner.UNIQUE:
		if tok, pos, lit := p.ScanIgnoreWhitespace(); tok != scanner.INDEX {
			return nil, newParseError(scanner.Tokstr(tok, lit), []string{"INDEX"}, pos)
		}

		return p.parseCreateIndexStatement(true)
	case scanner.INDEX:
		return p.parseCreateIndexStatement(false)
	}

	return nil, newParseError(scanner.Tokstr(tok, lit), []string{"TABLE", "INDEX"}, pos)
}

// parseCreateTableStatement parses a create table string and returns a Statement AST object.
// This function assumes the CREATE TABLE tokens have already been consumed.
func (p *parser) parseCreateTableStatement() (createTableStmt, error) {
	var stmt createTableStmt
	var err error

	// Parse IF NOT EXISTS
	stmt.ifNotExists, err = p.parseIfNotExists()
	if err != nil {
		return stmt, err
	}

	// Parse table name
	stmt.tableName, err = p.ParseIdent()
	if err != nil {
		return stmt, err
	}

	// parse primary key
	stmt.primaryKeyName, stmt.primaryKeyType, err = p.parseTableOptions()
	if err != nil {
		return stmt, err
	}

	return stmt, nil
}

func (p *parser) parseIfNotExists() (bool, error) {
	// Parse "IF"
	if tok, _, _ := p.ScanIgnoreWhitespace(); tok != scanner.IF {
		p.Unscan()
		return false, nil
	}

	// Parse "NOT"
	if tok, pos, lit := p.ScanIgnoreWhitespace(); tok != scanner.NOT {
		return false, newParseError(scanner.Tokstr(tok, lit), []string{"NOT", "EXISTS"}, pos)
	}

	// Parse "EXISTS"
	if tok, pos, lit := p.ScanIgnoreWhitespace(); tok != scanner.EXISTS {
		return false, newParseError(scanner.Tokstr(tok, lit), []string{"EXISTS"}, pos)
	}

	return true, nil
}

func (p *parser) parseTableOptions() (string, value.Type, error) {
	// Parse ( token.
	if tok, _, _ := p.ScanIgnoreWhitespace(); tok != scanner.LPAREN {
		p.Unscan()
		return "", 0, nil
	}

	keyName, err := p.ParseIdent()
	if err != nil {
		return "", 0, err
	}

	tp, err := p.parseType()
	if err != nil {
		return "", 0, err
	}

	// Parse "PRIMARY"
	if tok, pos, lit := p.ScanIgnoreWhitespace(); tok != scanner.PRIMARY {
		return "", 0, newParseError(scanner.Tokstr(tok, lit), []string{"PRIMARY", "KEY"}, pos)
	}

	// Parse "KEY"
	if tok, pos, lit := p.ScanIgnoreWhitespace(); tok != scanner.KEY {
		return "", 0, newParseError(scanner.Tokstr(tok, lit), []string{"KEY"}, pos)
	}

	// Parse required ) token.
	if tok, pos, lit := p.ScanIgnoreWhitespace(); tok != scanner.RPAREN {
		return "", 0, newParseError(scanner.Tokstr(tok, lit), []string{")"}, pos)
	}

	return keyName, tp, nil
}

// parseCreateIndexStatement parses a create index string and returns a Statement AST object.
// This function assumes the CREATE INDEX or CREATE UNIQUE INDEX tokens have already been consumed.
func (p *parser) parseCreateIndexStatement(unique bool) (createIndexStmt, error) {
	var err error
	stmt := createIndexStmt{
		unique: unique,
	}

	// Parse "IF"
	if tok, _, _ := p.ScanIgnoreWhitespace(); tok == scanner.IF {
		// Parse "NOT"
		if tok, pos, lit := p.ScanIgnoreWhitespace(); tok != scanner.NOT {
			return stmt, newParseError(scanner.Tokstr(tok, lit), []string{"NOT", "EXISTS"}, pos)
		}

		// Parse "EXISTS"
		if tok, pos, lit := p.ScanIgnoreWhitespace(); tok != scanner.EXISTS {
			return stmt, newParseError(scanner.Tokstr(tok, lit), []string{"EXISTS"}, pos)
		}

		stmt.ifNotExists = true
	} else {
		p.Unscan()
	}

	// Parse index name
	stmt.indexName, err = p.ParseIdent()
	if err != nil {
		return stmt, err
	}

	// Parse "ON"
	if tok, pos, lit := p.ScanIgnoreWhitespace(); tok != scanner.ON {
		return stmt, newParseError(scanner.Tokstr(tok, lit), []string{"ON"}, pos)
	}

	// Parse table name
	stmt.tableName, err = p.ParseIdent()
	if err != nil {
		return stmt, err
	}

	fields, ok, err := p.parseFieldList()
	if err != nil {
		return stmt, err
	}
	if !ok {
		tok, pos, lit := p.ScanIgnoreWhitespace()
		return stmt, newParseError(scanner.Tokstr(tok, lit), []string{"("}, pos)
	}

	if len(fields) != 1 {
		return stmt, &ParseError{Message: "indexes on more than one field not supported"}
	}

	stmt.fieldName = fields[0]

	return stmt, nil
}
