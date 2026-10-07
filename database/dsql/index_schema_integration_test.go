package dsql

import (
	"strings"
	"testing"
)

// The table is outside the driver's bookkeeping schema, as in Todo. A skipped
// index must be checked in its own schema, including after a failed unique build.
func TestIntegrationExistingIndexInAnotherSchema(t *testing.T) {
	driver := openScratch(t, "x-occ-max-retries=3")
	prefix := testPrefix(t)
	table := `"public"."` + prefix + `_users"`
	index := prefix + "_unique"
	mustExec(t, driver, `CREATE TABLE `+table+` (id INT PRIMARY KEY, email TEXT)`)
	t.Cleanup(func() { mustExec(t, driver, `DROP TABLE IF EXISTS `+table) })
	mustExec(t, driver, `INSERT INTO `+table+` VALUES (1,'duplicate'),(2,'duplicate')`)
	if err := driver.Run(strings.NewReader(`CREATE UNIQUE INDEX ASYNC ` + index + ` ON ` + table + ` (email)`)); err == nil {
		t.Fatal("unique build accepted duplicate data")
	}
	err := driver.Run(strings.NewReader(`CREATE UNIQUE INDEX ASYNC IF NOT EXISTS ` + index + ` ON ` + table + ` (email)`))
	if err == nil || !strings.Contains(err.Error(), "INVALID") {
		t.Fatalf("invalid index outside metadata schema was accepted: %v", err)
	}
	mustExec(t, driver, `DROP INDEX "public"."`+index+`"`)
	mustExec(t, driver, `DELETE FROM `+table+` WHERE id=2`)
	statement := `CREATE UNIQUE INDEX ASYNC IF NOT EXISTS ` + index + ` ON ` + table + ` (email)`
	if err := driver.Run(strings.NewReader(statement)); err != nil {
		t.Fatal(err)
	}
	if err := driver.Run(strings.NewReader(statement)); err != nil {
		t.Fatalf("valid skipped index rejected: %v", err)
	}
}
