package dsql

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIndexBatchConfig(t *testing.T) {
	for _, size := range []int{0, 1, 4} {
		c := mustParseConfig(t, "dsql://admin@host/postgres?x-multi-statement=true&x-async-index-batch-size="+strconv.Itoa(size))
		if c.AsyncIndexBatchSize != size {
			t.Fatalf("size = %d, want %d", c.AsyncIndexBatchSize, size)
		}
		if err := c.validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, options := range []string{"x-async-index-batch-size=-1", "x-async-index-batch-size=abc"} {
		if _, err := (&DSQL{}).Open("dsql://admin@host/postgres?" + options); err == nil {
			t.Fatalf("accepted %s", options)
		}
	}
	for _, c := range []Config{{AsyncIndexBatchSize: -1}, {AsyncIndexBatchSize: 4}} {
		if driver, err := WithInstance(nil, &c); err == nil || driver != nil {
			t.Fatalf("accepted invalid config: %+v", c)
		}
	}
}

func TestRunIndexBatches(t *testing.T) {
	const a = "CREATE INDEX ASYNC a ON items (a);"
	const b = "CREATE UNIQUE INDEX ASYNC b ON items (b);"
	const c = "CREATE INDEX ASYNC c ON items (c);"
	const next = "CREATE TABLE child (id INT PRIMARY KEY, b INT REFERENCES items(b));"
	tests := []struct {
		name    string
		size    int
		sql     string
		setup   func(sqlmock.Sqlmock)
		wantErr string
	}{
		{"overlap_then_dependency", 4, a + b + next, func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			expectIndex(m, b, "b")
			expectWait(m, "a")
			expectWait(m, "b")
			m.ExpectExec(next).WillReturnResult(sqlmock.NewResult(0, 0))
		}, ""},
		{"bound_and_end_of_file", 2, a + b + c, func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			expectIndex(m, b, "b")
			expectWait(m, "a")
			expectWait(m, "b")
			expectIndex(m, c, "c")
			expectWait(m, "c")
		}, ""},
		{"default_is_sequential", 0, a + b, func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			expectWait(m, "a")
			expectIndex(m, b, "b")
			expectWait(m, "b")
		}, ""},
		{"one_is_sequential", 1, a + b, func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			expectWait(m, "a")
			expectIndex(m, b, "b")
			expectWait(m, "b")
		}, ""},
		{"submission_error_drains_accepted_jobs", 4, a + b + c, func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			m.ExpectQuery(b).WillReturnError(errors.New("submission rejected"))
			expectWait(m, "a")
		}, "submission rejected"},
		{"build_error_drains_group_and_stops", 4, a + b + next, func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			expectIndex(m, b, "b")
			m.ExpectQuery("CALL sys.wait_for_job($1)").WithArgs("a").WillReturnRows(sqlmock.NewRows([]string{"success"}).AddRow(false))
			m.ExpectQuery("SELECT status, details FROM sys.jobs WHERE job_id = $1").WithArgs("a").WillReturnRows(sqlmock.NewRows([]string{"status", "details"}).AddRow("failed", "index build failed"))
			expectWait(m, "b")
		}, "index build failed"},
		{"parse_error_drains_accepted_jobs", 4, a + strings.Repeat("x", 9000), func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			expectWait(m, "a")
		}, "token too long"},
		{"retry_submission_not_accepted_job", 4, a + b, func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			m.ExpectQuery(b).WillReturnError(&pgconn.PgError{Code: "40001"})
			expectIndex(m, b, "b")
			expectWait(m, "a")
			expectWait(m, "b")
		}, ""},
		{"if_not_exists_waits_for_previous_build", 4, a + "CREATE INDEX ASYNC IF NOT EXISTS a ON items (a);", func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			expectWait(m, "a")
			// Returning an id exercises a fresh IF NOT EXISTS build. The existing-index
			// catalog validity check has separate integration coverage.
			expectIndex(m, "CREATE INDEX ASYNC IF NOT EXISTS a ON items (a);", "second")
			expectWait(m, "second")
		}, ""},
		{"constraint_validation_is_a_barrier", 4, a + "ALTER TABLE ASYNC items VALIDATE CONSTRAINT valid_item;", func(m sqlmock.Sqlmock) {
			expectIndex(m, a, "a")
			expectWait(m, "a")
			expectIndex(m, "ALTER TABLE ASYNC items VALIDATE CONSTRAINT valid_item;", "validation")
			expectWait(m, "validation")
		}, ""},
		{"comments_preserve_batching", 4, "/* first */ " + a + "\n-- second\n" + b, func(m sqlmock.Sqlmock) {
			expectIndex(m, "/* first */ "+a, "a")
			expectIndex(m, "\n-- second\n"+b, "b")
			expectWait(m, "a")
			expectWait(m, "b")
		}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, m, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Close() })
			cfg := &Config{MultiStatementEnabled: true, MultiStatementMaxSize: 4096, AsyncIndexBatchSize: tc.size, OCCMaxRetries: 1, OCCMaxRetryDelay: 1}
			cfg.setDefaults()
			d := &DSQL{db: db, config: cfg}
			tc.setup(m)
			err = d.Run(strings.NewReader(tc.sql))
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if err = m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func expectIndex(m sqlmock.Sqlmock, statement, id string) {
	m.ExpectQuery(statement).WillReturnRows(sqlmock.NewRows([]string{"job_id"}).AddRow(id))
}
func expectWait(m sqlmock.Sqlmock, id string) {
	m.ExpectQuery("CALL sys.wait_for_job($1)").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"success"}).AddRow(true))
}
