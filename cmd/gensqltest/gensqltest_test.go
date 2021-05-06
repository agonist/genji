package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var program = `
-- setup:
CREATE TABLE foo (a int);
CREATE TABLE bar;

-- test: insert something
INSERT INTO foo (a) VALUES (1);

SELECT * FROM foo;
-- result:
{
  "a": 1
}

SELECT a, b FROM foo;
-- result:
{
  "a": 1,
  "b": null
}

SELECT z FROM foo;
-- result:
{"z": null}

-- test: something else
INSERT INTO foo (c) VALUES (3);
SELECT * FROM foo;
-- result:
{"c": 3}
`

func TestParse(t *testing.T) {
	r := strings.NewReader(program)
	ts := parse(r, "foobar", "foobar.sqlexpr")

	// setup
	require.Equal(t, "CREATE TABLE foo (a int);", ts.Setup[0])
	require.Equal(t, "CREATE TABLE bar;", ts.Setup[1])

	// test "insert something"
	require.Equal(t, "insert something", ts.Tests[0].Name)

	// first block
	stmt := ts.Tests[0].Statements[0]
	require.Equal(t, "INSERT INTO foo (a) VALUES (1);", stmt.Expr[0])
	require.Equal(t, "SELECT * FROM foo;", stmt.Expr[1])
	want := []string{
		`{`,
		`  "a": 1`,
		`}`,
	}
	require.Equal(t, want, stmt.Result)

	// second block
	stmt = ts.Tests[0].Statements[1]
	require.Equal(t, "SELECT a, b FROM foo;", stmt.Expr[0])
	want = []string{
		`{`,
		`  "a": 1,`,
		`  "b": null`,
		`}`,
	}
	require.Equal(t, want, stmt.Result)

	// third block
	stmt = ts.Tests[0].Statements[2]
	require.Equal(t, "SELECT z FROM foo;", stmt.Expr[0])
	want = []string{`{"z": null}`}
	require.Equal(t, want, stmt.Result)

	// test "something else"
	require.Equal(t, "something else", ts.Tests[1].Name)

	// first block
	stmt = ts.Tests[1].Statements[0]
	require.Equal(t, "INSERT INTO foo (c) VALUES (3);", stmt.Expr[0])
	require.Equal(t, "SELECT * FROM foo;", stmt.Expr[1])
	want = []string{`{"c": 3}`}
	require.Equal(t, want, stmt.Result)
}

func TestGegenerate(t *testing.T) {
	r := strings.NewReader(program)
	ts := parse(r, "foobar", "foobar.sqlexpr")

	var out bytes.Buffer

	err := generate(ts, "Something", &out)
	require.NoError(t, err)

	fmt.Println(out.String())
}
