package dbtesting_test

import (
	"context"
	"github.com/jwilner/dbtesting"
	"os"
	"testing"

	// include PQ postgres driver
	_ "github.com/lib/pq"
)

func TestMain(m *testing.M) {
	os.Exit(dbtesting.RunTests(m, dbtesting.Config{
		SetUpFunc: dbtesting.SQL(`
CREATE TABLE films (
    code        char(5) CONSTRAINT firstkey PRIMARY KEY,
    title       varchar(40) NOT NULL,
    did         integer NOT NULL,
    date_prod   date,
    kind        varchar(10),
    len         interval hour to minute
);
`),
		CleanUpFunc: dbtesting.SQL(`
DROP TABLE films;
`),
	}))
}

func TestPretend(t *testing.T) {
	t.Run("pretend", dbtesting.Inject(func(t *dbtesting.T) {
		var code, title = "abcde", "random title"

		if _, err := t.Tx.ExecContext(
			context.Background(),
			`INSERT INTO films (code, title, did) VALUES ($1, $2, 1);`,
			code,
			title,
		); err != nil {
			t.Fatalf("error inserting: %v", err)
		}

		res, err := t.Tx.QueryContext(context.Background(), `SELECT code, title FROM films;`)
		if err != nil {
			t.Fatalf("error performing read: %v", err)
		}

		if !res.Next() {
			t.Fatal("there should've been a row to read")
		}

		var resCode, resTitle string
		if err := res.Scan(&resCode, &resTitle); err != nil {
			t.Fatalf("Unable to scan from row: %v", err)
		}
		if code != resCode {
			t.Fatalf("didn't find the expected code: %#v, %#v", code, resCode)
		}
		if title != resTitle {
			t.Fatalf("didn't find the expected title: %#v, %#v", title, resTitle)
		}

		if res.Next() {
			t.Fatal("there was unexpectedly another row to read")
		}
	}))
}
