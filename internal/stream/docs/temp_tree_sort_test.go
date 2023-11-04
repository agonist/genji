package docs_test

import (
	"testing"

	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/internal/environment"
	"github.com/genjidb/genji/internal/expr"
	"github.com/genjidb/genji/internal/sql/parser"
	"github.com/genjidb/genji/internal/stream"
	"github.com/genjidb/genji/internal/stream/docs"
	"github.com/genjidb/genji/internal/stream/table"
	"github.com/genjidb/genji/internal/testutil"
	"github.com/genjidb/genji/internal/testutil/assert"
	"github.com/genjidb/genji/types"
	"github.com/stretchr/testify/require"
)

func TestTempTreeSort(t *testing.T) {
	tests := []struct {
		name     string
		sortExpr expr.Expr
		values   []types.Document
		want     []types.Document
		fails    bool
		desc     bool
	}{
		{
			"ASC",
			parser.MustParseExpr("a"),
			[]types.Document{
				testutil.MakeDocument(t, `{"a": 0}`),
				testutil.MakeDocument(t, `{"a": null}`),
				testutil.MakeDocument(t, `{"a": true}`),
			},
			[]types.Document{
				testutil.MakeDocument(t, `{}`),
				testutil.MakeDocument(t, `{"a": 0}`),
				testutil.MakeDocument(t, `{"a": 1}`),
			},
			false,
			false,
		},
		{
			"DESC",
			parser.MustParseExpr("a"),
			[]types.Document{
				testutil.MakeDocument(t, `{"a": 0}`),
				testutil.MakeDocument(t, `{"a": null}`),
				testutil.MakeDocument(t, `{"a": true}`),
			},
			[]types.Document{
				testutil.MakeDocument(t, `{"a": 1}`),
				testutil.MakeDocument(t, `{"a": 0}`),
				testutil.MakeDocument(t, `{}`),
			},
			false,
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, tx, cleanup := testutil.NewTestTx(t)
			defer cleanup()

			testutil.MustExec(t, db, tx, "CREATE TABLE test(a int)")

			for _, doc := range test.values {
				testutil.MustExec(t, db, tx, "INSERT INTO test VALUES ?", environment.Param{Value: doc})
			}

			var env environment.Environment
			env.DB = db
			env.Tx = tx

			s := stream.New(table.Scan("test"))
			if test.desc {
				s = s.Pipe(docs.TempTreeSortReverse(test.sortExpr))
			} else {
				s = s.Pipe(docs.TempTreeSort(test.sortExpr))
			}

			var got []types.Document
			err := s.Iterate(&env, func(env *environment.Environment) error {
				d, ok := env.GetDocument()
				require.True(t, ok)

				fb := document.NewFieldBuffer()
				fb.Copy(d)
				got = append(got, fb)
				return nil
			})

			if test.fails {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.Equal(t, len(got), len(test.want))
				for i := range got {
					testutil.RequireDocEqual(t, test.want[i], got[i])
				}
			}
		})
	}

	t.Run("String", func(t *testing.T) {
		require.Equal(t, `docs.TempTreeSort(a)`, docs.TempTreeSort(parser.MustParseExpr("a")).String())
	})
}
