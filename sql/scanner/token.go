package scanner

import (
	"strings"
)

// Token is a lexical token of the Genji SQL language.
type Token int

// These are a comprehensive list of Genji SQL language tokens.
const (
	// ILLEGAL Token, EOF, WS are Special Genji SQL tokens.
	ILLEGAL Token = iota
	EOF
	WS
	COMMENT

	// IDENT and the following are Genji SQL literal tokens.
	IDENT           // main
	NAMEDPARAM      // $param
	POSITIONALPARAM // ?
	NUMBER          // 12345.67
	INTEGER         // 12345
	STRING          // "abc"
	BADSTRING       // "abc
	BADESCAPE       // \q
	TRUE            // true
	FALSE           // false
	NULL            // NULL
	REGEX           // Regular expressions
	BADREGEX        // `.*
	literalEnd

	operatorBeg
	// ADD and the following are Genji SQL Operators
	ADD        // +
	SUB        // -
	MUL        // *
	DIV        // /
	MOD        // %
	BITWISEAND // &
	BITWISEOR  // |
	BITWISEXOR // ^
	BETWEEN    // BETWEEN

	AND // AND
	OR  // OR

	EQ       // =
	NEQ      // !=
	EQREGEX  // =~
	NEQREGEX // !~
	LT       // <
	LTE      // <=
	GT       // >
	GTE      // >=
	IN       // IN
	NIN      // NOT IN
	IS       // IS
	ISN      // IS NOT
	LIKE     // LIKE
	CONCAT   // ||
	operatorEnd

	LPAREN      // (
	RPAREN      // )
	LBRACKET    // {
	RBRACKET    // }
	LSBRACKET   // [
	RSBRACKET   // ]
	COMMA       // ,
	COLON       // :
	DOUBLECOLON // ::
	SEMICOLON   // ;
	DOT         // .

	keywordBeg
	// ALL and the following are Genji SQL Keywords
	ADD_KEYWORD
	ALTER
	AS
	ASC
	BEGIN
	BY
	CAST
	COMMIT
	CREATE
	DEFAULT
	DELETE
	DESC
	DISTINCT
	DROP
	EXISTS
	EXPLAIN
	FIELD
	FROM
	GROUP
	IF
	INDEX
	INSERT
	INTO
	KEY
	LIMIT
	NOT
	OFFSET
	ON
	ONLY
	ORDER
	PRECISION
	PRIMARY
	READ
	REINDEX
	RENAME
	RETURNING
	ROLLBACK
	SELECT
	SET
	TABLE
	TO
	TRANSACTION
	UNIQUE
	UNSET
	UPDATE
	VALUES
	WHERE
	WRITE

	// Aliases
	TYPEARRAY
	TYPEBIGINT
	TYPEBLOB
	TYPEBOOL
	TYPEBYTES
	TYPECHARACTER
	TYPEDOCUMENT
	TYPEDOUBLE
	TYPEINT
	TYPEINT2
	TYPEINT8
	TYPEINTEGER
	TYPEMEDIUMINT
	TYPESMALLINT
	TYPETEXT
	TYPETINYINT
	TYPEREAL
	TYPEVARCHAR

	keywordEnd
)

var tokens = [...]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	WS:      "WS",

	IDENT:           "IDENT",
	POSITIONALPARAM: "?",
	NUMBER:          "NUMBER",
	STRING:          "STRING",
	BADSTRING:       "BADSTRING",
	BADESCAPE:       "BADESCAPE",
	TRUE:            "TRUE",
	FALSE:           "FALSE",
	REGEX:           "REGEX",
	NULL:            "NULL",

	ADD:        "+",
	SUB:        "-",
	MUL:        "*",
	DIV:        "/",
	MOD:        "%",
	BITWISEAND: "&",
	BITWISEOR:  "|",
	BITWISEXOR: "^",
	BETWEEN:    "BETWEEN",

	AND: "AND",
	OR:  "OR",

	EQ:       "=",
	NEQ:      "!=",
	EQREGEX:  "=~",
	NEQREGEX: "!~",
	LT:       "<",
	LTE:      "<=",
	GT:       ">",
	GTE:      ">=",
	IN:       "IN",
	IS:       "IS",
	LIKE:     "LIKE",

	LPAREN:      "(",
	RPAREN:      ")",
	LBRACKET:    "{",
	RBRACKET:    "}",
	LSBRACKET:   "[",
	RSBRACKET:   "]",
	COMMA:       ",",
	COLON:       ":",
	DOUBLECOLON: "::",
	SEMICOLON:   ";",
	DOT:         ".",

	ADD_KEYWORD: "ADD",
	ALTER:       "ALTER",
	AS:          "AS",
	ASC:         "ASC",
	BEGIN:       "BEGIN",
	COMMIT:      "COMMIT",
	GROUP:       "GROUP",
	BY:          "BY",
	CREATE:      "CREATE",
	CAST:        "CAST",
	DEFAULT:     "DEFAULT",
	DELETE:      "DELETE",
	DESC:        "DESC",
	DISTINCT:    "DISTINCT",
	DROP:        "DROP",
	EXISTS:      "EXISTS",
	EXPLAIN:     "EXPLAIN",
	KEY:         "KEY",
	FIELD:       "FIELD",
	FROM:        "FROM",
	IF:          "IF",
	INDEX:       "INDEX",
	INSERT:      "INSERT",
	INTO:        "INTO",
	LIMIT:       "LIMIT",
	NOT:         "NOT",
	OFFSET:      "OFFSET",
	ON:          "ON",
	ONLY:        "ONLY",
	ORDER:       "ORDER",
	PRECISION:   "PRECISION",
	PRIMARY:     "PRIMARY",
	READ:        "READ",
	REINDEX:     "REINDEX",
	RENAME:      "RENAME",
	RETURNING:   "RETURNING",
	ROLLBACK:    "ROLLBACK",
	SELECT:      "SELECT",
	SET:         "SET",
	TABLE:       "TABLE",
	TO:          "TO",
	TRANSACTION: "TRANSACTION",
	UNIQUE:      "UNIQUE",
	UNSET:       "UNSET",
	UPDATE:      "UPDATE",
	VALUES:      "VALUES",
	WHERE:       "WHERE",
	WRITE:       "WRITE",

	TYPEARRAY:     "ARRAY",
	TYPEBIGINT:    "BIGINT",
	TYPEBLOB:      "BLOB",
	TYPEBOOL:      "BOOL",
	TYPEBYTES:     "BYTES",
	TYPECHARACTER: "CHARACTER",
	TYPEDOCUMENT:  "DOCUMENT",
	TYPEDOUBLE:    "DOUBLE",
	TYPEINT:       "INT",
	TYPEINT2:      "INT2",
	TYPEINT8:      "INT8",
	TYPEINTEGER:   "INTEGER",
	TYPEMEDIUMINT: "MEDIUMINT",
	TYPESMALLINT:  "SMALLINT",
	TYPETEXT:      "TEXT",
	TYPETINYINT:   "TINYINT",
	TYPEREAL:      "REAL",
	TYPEVARCHAR:   "VARCHAR",
}

var keywords map[string]Token

// String returns the string representation of the token.
func (tok Token) String() string {
	if tok >= 0 && tok < Token(len(tokens)) {
		return tokens[tok]
	}
	return ""
}

// Precedence returns the operator precedence of the binary operator token.
func (tok Token) Precedence() int {
	switch tok {
	case OR:
		return 1
	case AND:
		return 2
	case EQ, NEQ, IS, IN, LIKE, EQREGEX, NEQREGEX, BETWEEN:
		return 3
	case LT, LTE, GT, GTE:
		return 4
	case BITWISEOR, BITWISEXOR, BITWISEAND:
		return 5
	case ADD, SUB:
		return 6
	case MUL, DIV, MOD:
		return 7
	case CONCAT:
		return 8
	}
	return 0
}

// IsOperator returns true for operator tokens.
func (tok Token) IsOperator() bool { return tok > operatorBeg && tok < operatorEnd }

// Tokstr returns a literal if provided, otherwise returns the token string.
func Tokstr(tok Token, lit string) string {
	if lit != "" {
		return lit
	}
	return tok.String()
}

// Lookup returns the token associated with a given string.
func Lookup(ident string) Token {
	if tok, ok := keywords[strings.ToLower(ident)]; ok {
		return tok
	}
	return IDENT
}

// Pos specifies the line and character position of a token.
// The Char and Line are both zero-based indexes.
type Pos struct {
	Line int
	Char int
}
