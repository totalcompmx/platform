//go:build integration

package database

import (
	"fmt"
	"os"
	"strings"
	"time"

	"testing"
)

type testUser struct {
	id             int
	email          string
	password       string
	hashedPassword string
}

var testUsers = map[string]*testUser{
	"alice": {email: "alice@example.com", password: "testPass123!", hashedPassword: "$2a$04$mi5gstbTPDRpEawTIitij.rdzLFM.U8.x4U5LLzK8xVFXKXf2ng2u"},
	"bob":   {email: "bob@example.com", password: "mySecure456#", hashedPassword: "$2a$04$AG864hNeosMGVOZKBePuRejH7ElpHfFBBHTFS6/XFJS4beixwXZB."},
}

func newTestDB(t *testing.T) *DB {
	t.Helper()

	dsn := testDBDSN(t)
	schemaName := newTestSchemaName()
	db := openTestDB(t, testSchemaDSN(dsn, schemaName))

	registerTestDBCleanup(t, db, schemaName)
	createTestSchema(t, db, schemaName)
	migrateTestDB(t, db)
	seedTestUsers(t, db)

	return db
}

func testDBDSN(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv("TEST_DB_DSN")

	if dsn == "" {
		t.Skip("TEST_DB_DSN environment variable must be set to run integration tests")
	}

	return dsn
}

func newTestSchemaName() string {
	return fmt.Sprintf("test_schema_%d", time.Now().UnixNano())
}

func testSchemaDSN(dsn, schemaName string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}

	return fmt.Sprintf("%s%ssearch_path=%s,public", dsn, separator, schemaName)
}

func openTestDB(t *testing.T, dsn string) *DB {
	t.Helper()

	db, err := New(dsn)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func registerTestDBCleanup(t *testing.T, db *DB, schemaName string) {
	t.Helper()

	t.Cleanup(func() {
		defer db.Close()

		_, err := db.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
		if err != nil {
			t.Error(err)
		}
	})
}

func createTestSchema(t *testing.T, db *DB, schemaName string) {
	t.Helper()

	_, err := db.Exec(fmt.Sprintf("CREATE SCHEMA %s", schemaName))
	if err != nil {
		t.Fatal(err)
	}
}

func migrateTestDB(t *testing.T, db *DB) {
	t.Helper()

	err := db.MigrateUp()
	if err != nil {
		t.Fatal(err)
	}
}

func seedTestUsers(t *testing.T, db *DB) {
	t.Helper()

	for _, user := range testUsers {
		id, err := db.InsertUser(user.email, user.hashedPassword)
		if err != nil {
			t.Fatal(err)
		}

		user.id = id
	}
}
