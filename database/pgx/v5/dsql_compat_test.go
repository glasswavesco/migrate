package pgx

import (
	"strings"
	"testing"
)

func TestPostgresDSQL(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"CREATE INDEX ASYNC idx ON t(c);", "CREATE INDEX       idx ON t(c);"},
		{"create unique index /* keep ASYNC */ async idx ON t(c);", "create unique index /* keep ASYNC */       idx ON t(c);"},
		{"ALTER TABLE ASYNC t VALIDATE CONSTRAINT c;", "ALTER TABLE       t VALIDATE CONSTRAINT c;"},
		{"-- CREATE INDEX ASYNC fake;\nCREATE INDEX ASYNC i ON t(c);", "-- CREATE INDEX ASYNC fake;\nCREATE INDEX       i ON t(c);"},
		{"/* outer /* ASYNC */ inner */ CREATE INDEX ASYNC i ON t(c);", "/* outer /* ASYNC */ inner */ CREATE INDEX       i ON t(c);"},
		{"INSERT INTO t VALUES ('CREATE INDEX ASYNC; ALTER TABLE ASYNC', 'it''s ASYNC'); CREATE INDEX ASYNC i ON t(c);", "INSERT INTO t VALUES ('CREATE INDEX ASYNC; ALTER TABLE ASYNC', 'it''s ASYNC'); CREATE INDEX       i ON t(c);"},
		{"CREATE TABLE \"async\" (async TEXT DEFAULT $$CREATE INDEX ASYNC;$$);", "CREATE TABLE \"async\" (async TEXT DEFAULT $$CREATE INDEX ASYNC;$$);"},
		{"SELECT $body$CREATE INDEX ASYNC; ALTER TABLE ASYNC$body$; CREATE INDEX ASYNC i ON t(c);", "SELECT $body$CREATE INDEX ASYNC; ALTER TABLE ASYNC$body$; CREATE INDEX       i ON t(c);"},
		{`SELECT E'escaped\' CREATE INDEX ASYNC;'; CREATE INDEX ASYNC i ON t(c);`, `SELECT E'escaped\' CREATE INDEX ASYNC;'; CREATE INDEX       i ON t(c);`},
		{"SELECT 1; CREATE TABLE ASYNC t(c TEXT);", "SELECT 1; CREATE TABLE ASYNC t(c TEXT);"},
	} {
		got, err := postgresDSQL([]byte(tc.in))
		if err != nil || string(got) != tc.want {
			t.Fatalf("input %q: got %q, err %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestPostgresDSQLRejectsIncompleteSQL(t *testing.T) {
	for _, sql := range []string{"/*", "SELECT 'ASYNC", "SELECT $tag$ASYNC", `SELECT "async`} {
		if _, err := postgresDSQL([]byte(sql)); err == nil {
			t.Fatalf("accepted %q", sql)
		}
	}
}

func TestPostgresDSQLPreservesMigrationLength(t *testing.T) {
	in := "CREATE\nINDEX\nASYNC idx ON t(c);"
	out, err := postgresDSQL([]byte(in))
	if err != nil || len(out) != len(in) || strings.Count(string(out), "\n") != strings.Count(in, "\n") {
		t.Fatal("changed error offsets")
	}
}
