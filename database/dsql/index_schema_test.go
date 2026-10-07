package dsql

import "testing"

func TestIndexSchemaFromIfNotExists(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		{`CREATE UNIQUE INDEX ASYNC IF NOT EXISTS idx ON glasswaves_auth.users (email)`, "glasswaves_auth"},
		{`CREATE INDEX ASYNC IF NOT EXISTS "idx""z" ON "Auth"."users" (email)`, "Auth"},
		{`CREATE INDEX ASYNC IF NOT EXISTS idx ON "weird.schema".users (email)`, "weird.schema"},
		{`CREATE INDEX ASYNC IF NOT EXISTS idx ON "Auth""Schema".users (email)`, `Auth"Schema`},
		{`CREATE INDEX /*why*/ ASYNC IF NOT EXISTS idx ON ONLY Auth.users (email)`, "auth"},
		{`CREATE INDEX ASYNC IF NOT EXISTS idx ON users (email)`, ""},
	} {
		if got := indexSchemaFromIfNotExists(tc.sql); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.sql, got, tc.want)
		}
	}
}
