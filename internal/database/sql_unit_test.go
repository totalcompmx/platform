package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/jmoiron/sqlx"
)

var fakeDriverID atomic.Int64

func TestNewWithInjectedConnector(t *testing.T) {
	restore := stubConnectDB(func(ctx context.Context, driverName string, dataSourceName string) (*sqlx.DB, error) {
		return newFakeSQLXDB(t, &fakeSQL{}), nil
	})
	defer restore()

	db, err := New("user:pass@example/db")

	if err != nil {
		t.Fatal(err)
	}
	if db.dsn != "user:pass@example/db" {
		t.Fatalf("got dsn %q", db.dsn)
	}
	if db.Stats().MaxOpenConnections != 25 {
		t.Fatalf("connection pool was not configured")
	}
}

func TestNewReportsConnectorErrors(t *testing.T) {
	restore := stubConnectDB(func(ctx context.Context, driverName string, dataSourceName string) (*sqlx.DB, error) {
		return nil, errors.New("connect failed")
	})
	defer restore()

	db, err := New("bad")

	if err == nil {
		t.Fatal("expected connection error")
	}
	if db != nil {
		t.Fatalf("got db %v; want nil", db)
	}
}

func TestMigrateUpUnit(t *testing.T) {
	t.Run("runs migrations", testMigrateUpRunsMigrations)
	t.Run("reports source errors", testMigrateUpSourceError)
	t.Run("reports runner creation errors", testMigrateUpRunnerError)
	t.Run("ignores no change", testMigrateUpNoChange)
}

func TestDefaultMigrationRunnerReportsInvalidDriver(t *testing.T) {
	_, err := newMigrationRunner("iofs", nil, "postgres://unit")
	if err == nil {
		t.Fatal("expected invalid migration runner error")
	}
}

func testMigrateUpRunsMigrations(t *testing.T) {
	runner := &fakeMigrationRunner{}
	restore := stubMigrationSeams(nil, runner)
	defer restore()

	err := (&DB{dsn: "unit"}).MigrateUp()

	if err != nil {
		t.Fatal(err)
	}
	if !runner.ran {
		t.Fatal("migration runner was not called")
	}
}

func testMigrateUpSourceError(t *testing.T) {
	restore := stubMigrationSeams(errors.New("source failed"), &fakeMigrationRunner{})
	defer restore()

	err := (&DB{dsn: "unit"}).MigrateUp()

	assertErrorContains(t, err, "source failed")
}

func testMigrateUpRunnerError(t *testing.T) {
	restore := stubMigrationRunnerError(errors.New("runner failed"))
	defer restore()

	err := (&DB{dsn: "unit"}).MigrateUp()

	assertErrorContains(t, err, "runner failed")
}

func testMigrateUpNoChange(t *testing.T) {
	restore := stubMigrationSeams(nil, &fakeMigrationRunner{err: migrate.ErrNoChange})
	defer restore()

	err := (&DB{dsn: "unit"}).MigrateUp()

	if err != nil {
		t.Fatal(err)
	}
}

func TestPayrollQueries(t *testing.T) {
	t.Run("active fiscal year", testActiveFiscalYearQuery)
	t.Run("ISR brackets", testISRBracketsQuery)
	t.Run("IMSS concepts", testIMSSConceptsQuery)
	t.Run("cesantia bracket", testCesantiaBracketQuery)
	t.Run("RESICO bracket", testRESICOBracketQuery)
	t.Run("exchange rate update", testExchangeRateUpdate)
	t.Run("UMA upsert", testUMAUpsert)
}

func TestUserQueries(t *testing.T) {
	t.Run("insert user", testInsertUserQuery)
	t.Run("get user by ID", testGetUserQuery)
	t.Run("get user by email", testGetUserByEmailQuery)
	t.Run("update password", testUpdateUserPasswordQuery)
	t.Run("update API key", testUpdateUserAPIKeyQuery)
	t.Run("get user by API key", testGetUserByAPIKeyQuery)
	t.Run("increment API call count", testIncrementAPICallsCountQuery)
	t.Run("daily API call count", testDailyAPICallCountQuery)
	t.Run("log API call", testLogAPICallQuery)
	t.Run("insert verification token", testInsertVerificationTokenQuery)
	t.Run("get user ID from verification token", testGetUserIDFromVerificationTokenQuery)
	t.Run("verify user email", testVerifyUserEmailQuery)
	t.Run("delete verification tokens", testDeleteEmailVerificationTokensQuery)
}

func TestPasswordResetQueries(t *testing.T) {
	t.Run("insert password reset", testInsertPasswordResetQuery)
	t.Run("get password reset", testGetPasswordResetQuery)
	t.Run("delete password resets", testDeletePasswordResetsQuery)
}

func TestNoRowsBranches(t *testing.T) {
	db := newFakeDB(t, &fakeSQL{noRows: true})

	fy, found, err := db.GetActiveFiscalYear()
	assertNotFound(t, fy, found, err)
	user, found, err := db.GetUser(1)
	assertUserNotFound(t, user, found, err)
	user, found, err = db.GetUserByEmail("none@example.com")
	assertUserNotFound(t, user, found, err)
	user, found, err = db.GetUserByAPIKey("none")
	assertUserNotFound(t, user, found, err)
	id, found, err := db.GetUserIDFromVerificationToken("none")
	assertIDNotFound(t, id, found, err)
	reset, found, err := db.GetPasswordReset("none")
	assertResetNotFound(t, reset, found, err)
	if _, found, err := db.GetCesantiaBracket(1, 1); err != nil || found {
		t.Fatalf("GetCesantiaBracket found=%t err=%v", found, err)
	}
	if _, found, err := db.GetRESICOBracket(1, 1); err != nil || found {
		t.Fatalf("GetRESICOBracket found=%t err=%v", found, err)
	}
}

func testActiveFiscalYearQuery(t *testing.T) {
	_, found, err := newFakeDB(t, &fakeSQL{}).GetActiveFiscalYear()
	assertFound(t, "GetActiveFiscalYear", found, err)
}

func testISRBracketsQuery(t *testing.T) {
	_, err := newFakeDB(t, &fakeSQL{}).GetISRBrackets(1)
	if err != nil {
		t.Fatal(err)
	}
}

func testIMSSConceptsQuery(t *testing.T) {
	_, err := newFakeDB(t, &fakeSQL{}).GetIMSSConcepts()
	if err != nil {
		t.Fatal(err)
	}
}

func testCesantiaBracketQuery(t *testing.T) {
	_, found, err := newFakeDB(t, &fakeSQL{}).GetCesantiaBracket(1, 2)
	assertFound(t, "GetCesantiaBracket", found, err)
}

func testRESICOBracketQuery(t *testing.T) {
	_, found, err := newFakeDB(t, &fakeSQL{}).GetRESICOBracket(1, 5000)
	assertFound(t, "GetRESICOBracket", found, err)
}

func testExchangeRateUpdate(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).UpdateExchangeRate(20)
	if err != nil {
		t.Fatal(err)
	}
}

func testUMAUpsert(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).UpsertUMAForYear(2026, 1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
}

func testInsertUserQuery(t *testing.T) {
	id, err := newFakeDB(t, &fakeSQL{}).InsertUser("a@example.com", "hash")
	if err != nil || id != 123 {
		t.Fatalf("InsertUser id=%d err=%v", id, err)
	}
}

func testGetUserQuery(t *testing.T) {
	_, found, err := newFakeDB(t, &fakeSQL{}).GetUser(123)
	assertFound(t, "GetUser", found, err)
}

func testGetUserByEmailQuery(t *testing.T) {
	_, found, err := newFakeDB(t, &fakeSQL{}).GetUserByEmail("a@example.com")
	assertFound(t, "GetUserByEmail", found, err)
}

func testUpdateUserPasswordQuery(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).UpdateUserHashedPassword(123, "new")
	if err != nil {
		t.Fatal(err)
	}
}

func testUpdateUserAPIKeyQuery(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).UpdateUserAPIKey(123, "key")
	if err != nil {
		t.Fatal(err)
	}
}

func testGetUserByAPIKeyQuery(t *testing.T) {
	_, found, err := newFakeDB(t, &fakeSQL{}).GetUserByAPIKey("key")
	assertFound(t, "GetUserByAPIKey", found, err)
}

func testIncrementAPICallsCountQuery(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).IncrementAPICallsCount(123)
	if err != nil {
		t.Fatal(err)
	}
}

func testDailyAPICallCountQuery(t *testing.T) {
	count, err := newFakeDB(t, &fakeSQL{}).GetDailyAPICallCount(123)
	if err != nil || count != 7 {
		t.Fatalf("GetDailyAPICallCount count=%d err=%v", count, err)
	}
}

func testLogAPICallQuery(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).LogAPICall(123)
	if err != nil {
		t.Fatal(err)
	}
}

func testInsertVerificationTokenQuery(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).InsertEmailVerificationToken(123, "hash")
	if err != nil {
		t.Fatal(err)
	}
}

func testGetUserIDFromVerificationTokenQuery(t *testing.T) {
	id, found, err := newFakeDB(t, &fakeSQL{}).GetUserIDFromVerificationToken("hash")
	if err != nil || !found || id != 123 {
		t.Fatalf("GetUserIDFromVerificationToken id=%d found=%t err=%v", id, found, err)
	}
}

func testVerifyUserEmailQuery(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).VerifyUserEmail(123)
	if err != nil {
		t.Fatal(err)
	}
}

func testDeleteEmailVerificationTokensQuery(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).DeleteEmailVerificationTokensForUser(123)
	if err != nil {
		t.Fatal(err)
	}
}

func testInsertPasswordResetQuery(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).InsertPasswordReset("hash", 123, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
}

func testGetPasswordResetQuery(t *testing.T) {
	reset, found, err := newFakeDB(t, &fakeSQL{}).GetPasswordReset("hash")
	if err != nil || !found || reset.UserID != 123 {
		t.Fatalf("GetPasswordReset reset=%v found=%t err=%v", reset, found, err)
	}
}

func testDeletePasswordResetsQuery(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{}).DeletePasswordResets(123)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseErrorBranches(t *testing.T) {
	db := newFakeDB(t, &fakeSQL{queryErr: errors.New("query failed"), execErr: errors.New("exec failed")})

	assertHasError(t, db.GetActiveFiscalYear)
	isrBrackets, err := db.GetISRBrackets(1)
	assertSliceError(t, isrBrackets, err)
	imssConcepts, err := db.GetIMSSConcepts()
	assertSliceError(t, imssConcepts, err)
	if err := db.UpdateExchangeRate(1); err == nil {
		t.Fatal("expected exec error")
	}
}

func TestDatabaseScanAndRowsErrors(t *testing.T) {
	if _, err := newFakeDB(t, &fakeSQL{badScanFor: "from isr_brackets"}).GetISRBrackets(1); err == nil {
		t.Fatal("expected ISR scan error")
	}
	if _, err := newFakeDB(t, &fakeSQL{rowsErrFor: "from isr_brackets"}).GetISRBrackets(1); err == nil {
		t.Fatal("expected ISR rows error")
	}
	if _, err := newFakeDB(t, &fakeSQL{badScanFor: "from imss_concepts"}).GetIMSSConcepts(); err == nil {
		t.Fatal("expected IMSS scan error")
	}
	if _, err := newFakeDB(t, &fakeSQL{rowsErrFor: "from imss_concepts"}).GetIMSSConcepts(); err == nil {
		t.Fatal("expected IMSS rows error")
	}
}

func TestSpecificQueryErrors(t *testing.T) {
	if _, _, err := newFakeDB(t, &fakeSQL{queryErrFor: "from imss_employer_cesantia_brackets"}).GetCesantiaBracket(1, 1); err == nil {
		t.Fatal("expected cesantia query error")
	}
	if _, _, err := newFakeDB(t, &fakeSQL{queryErrFor: "from resico_brackets"}).GetRESICOBracket(1, 1); err == nil {
		t.Fatal("expected resico query error")
	}
	if _, err := newFakeDB(t, &fakeSQL{queryErrFor: "insert into users"}).InsertUser("a@example.com", "hash"); err == nil {
		t.Fatal("expected insert user error")
	}
}

func TestUpsertUMAErrorBranches(t *testing.T) {
	t.Run("insert fiscal year error", testUpsertUMAInsertError)
	t.Run("target fiscal year query error", testUpsertUMATargetQueryError)
	t.Run("source fiscal year query error", testUpsertUMASourceQueryError)
	t.Run("source fiscal year not found", testUpsertUMASourceNotFound)
	t.Run("clone bracket error", testUpsertUMACloneError)
	t.Run("deactivate fiscal years error", testUpsertUMADeactivateError)
}

func TestVerifyUserEmailErrorBranches(t *testing.T) {
	if err := newFakeDB(t, &fakeSQL{beginErr: errors.New("begin failed")}).VerifyUserEmail(123); err == nil {
		t.Fatal("expected begin error")
	}
	if err := newFakeDB(t, &fakeSQL{execErrFor: "update users set email_verified"}).VerifyUserEmail(123); err == nil {
		t.Fatal("expected update error")
	}
	if err := newFakeDB(t, &fakeSQL{execErrFor: "delete from email_verifications"}).VerifyUserEmail(123); err == nil {
		t.Fatal("expected delete error")
	}
	if err := newFakeDB(t, &fakeSQL{commitErr: errors.New("commit failed")}).VerifyUserEmail(123); err == nil {
		t.Fatal("expected commit error")
	}
}

func TestRowsAffectedErrors(t *testing.T) {
	if err := requireRowsAffected(fakeResult{rows: 0}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("got %v; want sql.ErrNoRows", err)
	}
	if err := requireRowsAffected(fakeResult{err: errors.New("rows failed")}); err == nil {
		t.Fatal("expected rows affected error")
	}
}

func testUpsertUMAInsertError(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{execErrFor: "insert into fiscal_years"}).UpsertUMAForYear(2026, 1, 2, 3)
	if err == nil {
		t.Fatal("expected upsert exec error")
	}
}

func testUpsertUMATargetQueryError(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{queryErrFor: "select id from fiscal_years where year"}).UpsertUMAForYear(2026, 1, 2, 3)
	if err == nil {
		t.Fatal("expected target fiscal year query error")
	}
}

func testUpsertUMASourceQueryError(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{queryErrFor: "order by year desc"}).UpsertUMAForYear(2026, 1, 2, 3)
	if err == nil {
		t.Fatal("expected source fiscal year query error")
	}
}

func testUpsertUMASourceNotFound(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{sourceNoRows: true}).UpsertUMAForYear(2026, 1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
}

func testUpsertUMACloneError(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{execErrFor: "insert into isr_brackets"}).UpsertUMAForYear(2026, 1, 2, 3)
	if err == nil {
		t.Fatal("expected clone exec error")
	}
}

func testUpsertUMADeactivateError(t *testing.T) {
	err := newFakeDB(t, &fakeSQL{execErrFor: "set is_active = false"}).UpsertUMAForYear(2026, 1, 2, 3)
	if err == nil {
		t.Fatal("expected deactivate exec error")
	}
}

func TestTransactionErrors(t *testing.T) {
	if err := newFakeDB(t, &fakeSQL{beginErr: errors.New("begin failed")}).withTx(context.Background(), func(tx *sql.Tx) error { return nil }); err == nil {
		t.Fatal("expected begin error")
	}
	if err := newFakeDB(t, &fakeSQL{}).withTx(context.Background(), func(tx *sql.Tx) error { return errors.New("fn failed") }); err == nil {
		t.Fatal("expected function error")
	}
	if err := newFakeDB(t, &fakeSQL{commitErr: errors.New("commit failed")}).withTx(context.Background(), func(tx *sql.Tx) error { return nil }); err == nil {
		t.Fatal("expected commit error")
	}
}

func newFakeDB(t *testing.T, fake *fakeSQL) *DB {
	return &DB{dsn: "unit", DB: newFakeSQLXDB(t, fake)}
}

func newFakeSQLXDB(t *testing.T, fake *fakeSQL) *sqlx.DB {
	driverName := fmt.Sprintf("fake-sql-%d", fakeDriverID.Add(1))
	sql.Register(driverName, fakeDriver{fake: fake})
	sqlDB, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return sqlx.NewDb(sqlDB, driverName)
}

type fakeSQL struct {
	queryErr     error
	execErr      error
	noRows       bool
	beginErr     error
	commitErr    error
	pingErr      error
	queryErrFor  string
	execErrFor   string
	badScanFor   string
	rowsErrFor   string
	sourceNoRows bool
}

type fakeDriver struct {
	fake *fakeSQL
}

func (d fakeDriver) Open(name string) (driver.Conn, error) {
	return fakeConn{fake: d.fake}, nil
}

type fakeConn struct {
	fake *fakeSQL
}

func (c fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c fakeConn) Close() error { return nil }
func (c fakeConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c fakeConn) Ping(context.Context) error { return c.fake.pingErr }

func (c fakeConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if c.fake.beginErr != nil {
		return nil, c.fake.beginErr
	}
	return fakeTx{fake: c.fake}, nil
}

func (c fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.queryError(query); err != nil {
		return nil, err
	}
	if c.emptyQueryResult(query) {
		return emptyRowsForQuery(query), nil
	}

	rows := rowsForQuery(query)
	c.applyBadScan(query, rows)
	c.applyRowsError(query, rows)
	return rows, nil
}

func (c fakeConn) queryError(query string) error {
	if c.fake.queryErr != nil {
		return c.fake.queryErr
	}
	if containsQuery(query, c.fake.queryErrFor) {
		return errors.New("query failed")
	}
	return nil
}

func (c fakeConn) emptyQueryResult(query string) bool {
	return c.fake.noRows || c.fake.sourceNoRows && containsQuery(query, "where year <>")
}

func (c fakeConn) applyBadScan(query string, rows *fakeRows) {
	if !containsQuery(query, c.fake.badScanFor) {
		return
	}
	rows.values[0][badScanColumn(query)] = "not-a-number"
}

func badScanColumn(query string) int {
	if strings.Contains(strings.ToLower(query), "from imss_concepts") {
		return 1
	}
	return 0
}

func (c fakeConn) applyRowsError(query string, rows *fakeRows) {
	if containsQuery(query, c.fake.rowsErrFor) {
		rows.err = errors.New("rows failed")
	}
}

func (c fakeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.fake.execErr != nil {
		return nil, c.fake.execErr
	}
	if containsQuery(query, c.fake.execErrFor) {
		return nil, errors.New("exec failed")
	}
	return fakeResult{rows: 1}, nil
}

type fakeTx struct {
	fake *fakeSQL
}

func (tx fakeTx) Commit() error {
	return tx.fake.commitErr
}

func (tx fakeTx) Rollback() error {
	return nil
}

type fakeRows struct {
	columns []string
	values  [][]driver.Value
	index   int
	err     error
}

func (r *fakeRows) Columns() []string {
	return r.columns
}

func (r *fakeRows) Close() error {
	return nil
}

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		if r.err != nil {
			err := r.err
			r.err = nil
			return err
		}
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func containsQuery(query string, pattern string) bool {
	return pattern != "" && strings.Contains(strings.ToLower(query), strings.ToLower(pattern))
}

func rowsForQuery(query string) *fakeRows {
	normalized := strings.ToLower(query)
	for _, spec := range fakeRowSpecs() {
		if spec.matches(normalized) {
			return row(spec.columns(), spec.values())
		}
	}
	return row([]string{"id"}, []driver.Value{int64(1)})
}

type fakeRowSpec struct {
	patterns []string
	columns  func() []string
	values   func() []driver.Value
}

func (spec fakeRowSpec) matches(query string) bool {
	for _, pattern := range spec.patterns {
		if !strings.Contains(query, pattern) {
			return false
		}
	}
	return true
}

func fakeRowSpecs() []fakeRowSpec {
	return []fakeRowSpec{
		newFakeRowSpec([]string{"from fiscal_years", "is_active = true"}, fiscalYearColumns, fiscalYearValues),
		fixedFakeRowSpec("from isr_brackets", []string{"lower_limit", "upper_limit", "fixed_fee", "surplus_percent"}, []driver.Value{0.01, 1000000.0, 0.0, 0.2}),
		fixedFakeRowSpec("from imss_concepts", []string{"concept_name", "worker_percent", "employer_percent", "base_cap_in_umas", "is_fixed_rate"}, []driver.Value{"Concept", 0.01, 0.02, int64(25), true}),
		fixedFakeRowSpec("from imss_employer_cesantia_brackets", []string{"lower_bound_uma", "upper_bound_uma", "employer_percent"}, []driver.Value{0.0, 25.0, 0.02}),
		fixedFakeRowSpec("from resico_brackets", []string{"upper_limit", "applicable_rate"}, []driver.Value{1000000.0, 0.015}),
		fixedFakeRowSpec("insert into users", []string{"id"}, []driver.Value{int64(123)}),
		newFakeRowSpec([]string{"select * from users"}, userColumns, userValues),
		fixedFakeRowSpec("select count(*)", []string{"count"}, []driver.Value{int64(7)}),
		fixedFakeRowSpec("select user_id", []string{"user_id"}, []driver.Value{int64(123)}),
		newFakeRowSpec([]string{"select * from password_resets"}, passwordResetColumns, passwordResetValues),
		fixedFakeRowSpec("select id from fiscal_years where year =", []string{"id"}, []driver.Value{int64(10)}),
		fixedFakeRowSpec("where year <>", []string{"id"}, []driver.Value{int64(9)}),
	}
}

func fixedFakeRowSpec(pattern string, columns []string, values []driver.Value) fakeRowSpec {
	return newFakeRowSpec([]string{pattern}, func() []string { return columns }, func() []driver.Value { return values })
}

func newFakeRowSpec(patterns []string, columns func() []string, values func() []driver.Value) fakeRowSpec {
	return fakeRowSpec{patterns: patterns, columns: columns, values: values}
}

func emptyRowsForQuery(query string) *fakeRows {
	rows := rowsForQuery(query)
	rows.values = nil
	return rows
}

func row(columns []string, values []driver.Value) *fakeRows {
	return &fakeRows{
		columns: append([]string(nil), columns...),
		values:  [][]driver.Value{append([]driver.Value(nil), values...)},
	}
}

func fiscalYearColumns() []string {
	return []string{"id", "year", "uma_daily", "uma_monthly", "uma_annual", "umi_value", "smg_general", "smg_border", "subsidy_factor", "subsidy_threshold_monthly", "fa_legal_cap_uma_factor", "fa_legal_max_percentage", "pantry_vouchers_uma_cap", "usd_mxn_rate"}
}

func fiscalYearValues() []driver.Value {
	return []driver.Value{int64(1), int64(2025), 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 0.1, 100.0, 1.3, 13.0, 1.0, 20.0}
}

func userColumns() []string {
	return []string{"id", "created", "email", "hashed_password", "api_key", "api_calls_count", "api_key_created_at", "email_verified", "email_verified_at"}
}

func userValues() []driver.Value {
	now := time.Now()
	return []driver.Value{int64(123), now, "a@example.com", "hash", "key", int64(7), now, true, now}
}

func passwordResetColumns() []string {
	return []string{"hashed_token", "user_id", "expiry"}
}

func passwordResetValues() []driver.Value {
	return []driver.Value{"hash", int64(123), time.Now().Add(time.Hour)}
}

type fakeResult struct {
	rows int64
	err  error
}

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return r.rows, r.err }

type fakeMigrationRunner struct {
	ran bool
	err error
}

func (r *fakeMigrationRunner) Up() error {
	r.ran = true
	return r.err
}

func stubConnectDB(fn func(context.Context, string, string) (*sqlx.DB, error)) func() {
	original := connectDB
	connectDB = fn
	return func() { connectDB = original }
}

func stubMigrationSeams(sourceErr error, runner migrationRunner) func() {
	originalSource := newMigrationSource
	originalRunner := newMigrationRunner
	newMigrationSource = func(fsys fs.FS, path string) (source.Driver, error) {
		return nil, sourceErr
	}
	newMigrationRunner = func(sourceName string, sourceDriver source.Driver, databaseURL string) (migrationRunner, error) {
		return runner, nil
	}
	return func() {
		newMigrationSource = originalSource
		newMigrationRunner = originalRunner
	}
}

func stubMigrationRunnerError(err error) func() {
	originalSource := newMigrationSource
	originalRunner := newMigrationRunner
	newMigrationSource = func(fsys fs.FS, path string) (source.Driver, error) {
		return nil, nil
	}
	newMigrationRunner = func(sourceName string, sourceDriver source.Driver, databaseURL string) (migrationRunner, error) {
		return nil, err
	}
	return func() {
		newMigrationSource = originalSource
		newMigrationRunner = originalRunner
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("got error %v; want containing %q", err, want)
	}
}

func assertFound(t *testing.T, name string, found bool, err error) {
	t.Helper()
	if err != nil || !found {
		t.Fatalf("%s found=%t err=%v", name, found, err)
	}
}

func assertNotFound(t *testing.T, _ FiscalYear, found bool, err error) {
	t.Helper()
	if err != nil || found {
		t.Fatalf("found=%t err=%v; want not found", found, err)
	}
}

func assertUserNotFound(t *testing.T, _ User, found bool, err error) {
	t.Helper()
	if err != nil || found {
		t.Fatalf("found=%t err=%v; want not found", found, err)
	}
}

func assertIDNotFound(t *testing.T, _ int, found bool, err error) {
	t.Helper()
	if err != nil || found {
		t.Fatalf("found=%t err=%v; want not found", found, err)
	}
}

func assertResetNotFound(t *testing.T, _ PasswordReset, found bool, err error) {
	t.Helper()
	if err != nil || found {
		t.Fatalf("found=%t err=%v; want not found", found, err)
	}
}

func assertHasError[T any](t *testing.T, fn func() (T, bool, error)) {
	t.Helper()
	_, _, err := fn()
	if err == nil {
		t.Fatal("expected error")
	}
}

func assertSliceError[T any](t *testing.T, _ []T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
}
