package repository

import (
	"database/sql"
	"testing"
)

// The DSN must refer to the disposable CI database; never use a business database.
func resetTestDatabase(t *testing.T, driver, dsn string) {
	t.Helper()
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if driver == "postgres" {
		if _, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
			t.Fatal(err)
		}
		return
	}
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(t.Context(), "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		t.Fatal(err)
	}
	rows, err := conn.QueryContext(t.Context(), "SHOW TABLES")
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for _, name := range tables {
		if _, err := conn.ExecContext(t.Context(), "DROP TABLE `"+name+"`"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.ExecContext(t.Context(), "SET FOREIGN_KEY_CHECKS=1"); err != nil {
		t.Fatal(err)
	}
}
