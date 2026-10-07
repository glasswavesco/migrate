package pgx

import (
	"strings"
	"testing"

	"github.com/dhui/dktest"
	"github.com/golang-migrate/migrate/v4/dktesting"
)

func TestDSQLCompatibilityUsingPostgres(t *testing.T) {
	dktesting.ParallelTest(t, []dktesting.ContainerSpec{{ImageName: "postgres:17", Options: opts}}, func(t *testing.T, c dktest.ContainerInfo) {
		host, port, err := c.FirstPort()
		if err != nil {
			t.Fatal(err)
		}
		driver, err := (&Postgres{}).Open(pgConnectionString(host, port, "x-dsql-compatibility=true"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := driver.Close(); err != nil {
				t.Error(err)
			}
		})
		for _, sql := range []string{
			`CREATE TABLE compat_example (id INT, value TEXT);`,
			`CREATE UNIQUE INDEX ASYNC compat_example_id ON compat_example(id);`,
			`INSERT INTO compat_example VALUES (1, 'ASYNC; CREATE INDEX ASYNC');`,
			`ALTER TABLE compat_example ADD CONSTRAINT compat_positive CHECK (id > 0) NOT VALID;`,
			`ALTER TABLE ASYNC compat_example VALIDATE CONSTRAINT compat_positive;`,
		} {
			if err := driver.Run(strings.NewReader(sql)); err != nil {
				t.Fatalf("run %q: %v", sql, err)
			}
		}
		pg := driver.(*Postgres)
		var value string
		if err := pg.db.QueryRow(`SELECT value FROM compat_example WHERE id=1`).Scan(&value); err != nil || value != "ASYNC; CREATE INDEX ASYNC" {
			t.Fatalf("literal changed: %q, %v", value, err)
		}
		pg.config.DSQLCompatibility = false
		if err := driver.Run(strings.NewReader(`CREATE INDEX ASYNC disabled_mode ON compat_example(value);`)); err == nil {
			t.Fatal("default PostgreSQL mode translated DSQL")
		}
	})
}
