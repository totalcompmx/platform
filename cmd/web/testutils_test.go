package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/smtp"

	"github.com/alexedwards/scs/v2"
	"github.com/andybalholm/cascadia"
	"github.com/justinas/nosurf"
	"golang.org/x/net/html"
)

type testUser struct {
	id             int
	email          string
	password       string
	hashedPassword string
}

var testUsers = map[string]*testUser{
	"alice": {email: "alice@example.com", password: "testPass123!", hashedPassword: "$2a$04$27fHaQw5jwiMKYoxhLek4uyj9zp29lxtmLWGuC0MR6tuispXJn9US"},
	"bob":   {email: "bob@example.com", password: "mySecure456#", hashedPassword: "$2a$04$O6QOPBSFw14SyLBXs64MJuQd8o7GaBKYvbDqeHGgZRi6FN87aXDWC"},
}

func newTestApplication(t *testing.T) *application {
	t.Helper()

	store := newFakeStore(t)
	app := new(application)
	app.config.session.cookieName = "session_test"
	app.config.baseURL = "https://example.com"
	app.config.cookie.secure = true
	app.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	app.db = store
	app.mailer = smtp.NewMockMailer("test@example.com")
	app.sessionManager = scs.New()
	app.sessionManager.Lifetime = 7 * 24 * time.Hour
	app.sessionManager.Cookie.Name = app.config.session.cookieName
	app.sessionManager.Cookie.Secure = true
	return app
}

type fakeStore struct {
	nextUserID        int
	users             map[int]database.User
	passwordResets    map[string]database.PasswordReset
	emailTokens       map[string]int
	apiCallCounts     map[int]int
	fiscalYear        database.FiscalYear
	isrBrackets       []database.ISRBracket
	imssConcepts      []database.IMSSConcept
	cesantiaBracket   database.CesantiaBracket
	resicoBracket     database.RESICOBracket
	activeFiscalFound bool
	cesantiaFound     bool
	resicoFound       bool
	errors            map[string]error
	errorOnCall       map[string]int
	calls             map[string]int
}

func newFakeStore(t *testing.T) *fakeStore {
	t.Helper()

	store := &fakeStore{
		nextUserID:        1,
		users:             map[int]database.User{},
		passwordResets:    map[string]database.PasswordReset{},
		emailTokens:       map[string]int{},
		apiCallCounts:     map[int]int{},
		fiscalYear:        testFiscalYear(),
		isrBrackets:       testISRBrackets(),
		imssConcepts:      testIMSSConcepts(),
		cesantiaBracket:   database.CesantiaBracket{LowerBoundUMA: 0, UpperBoundUMA: 25, EmployerPercent: 0.02},
		resicoBracket:     database.RESICOBracket{UpperLimit: 1000000, ApplicableRate: 0.015},
		activeFiscalFound: true,
		cesantiaFound:     true,
		resicoFound:       true,
		errors:            map[string]error{},
		errorOnCall:       map[string]int{},
		calls:             map[string]int{},
	}

	for _, user := range testUsers {
		id, err := store.InsertUser(user.email, user.hashedPassword)
		if err != nil {
			t.Fatal(err)
		}
		user.id = id
	}

	return store
}

func testFiscalYear() database.FiscalYear {
	return database.FiscalYear{
		ID:                      1,
		Year:                    2025,
		UMADaily:                113.14,
		UMAMonthly:              3439.46,
		UMAAnnual:               41373.52,
		UMIValue:                100.81,
		SMGGeneral:              278.80,
		SMGBorder:               419.88,
		SubsidyFactor:           0.1,
		SubsidyThresholdMonthly: 9000,
		FALegalCapUMAFactor:     1.3,
		FALegalMaxPercentage:    13,
		PantryVouchersUMACap:    1,
		USDMXNRate:              20,
	}
}

func testISRBrackets() []database.ISRBracket {
	return []database.ISRBracket{
		{LowerLimit: 0.01, UpperLimit: 10000, FixedFee: 0, SurplusPercent: 0.10},
		{LowerLimit: 10000.01, UpperLimit: 50000, FixedFee: 1000, SurplusPercent: 0.20},
		{LowerLimit: 50000.01, UpperLimit: 1000000, FixedFee: 9000, SurplusPercent: 0.30},
	}
}

func testIMSSConcepts() []database.IMSSConcept {
	return []database.IMSSConcept{
		{ConceptName: "Enfermedades y Maternidad", WorkerPercent: 0.004, EmployerPercent: 0.011, BaseCapInUMAs: 25, IsFixedRate: true},
		{ConceptName: "Cesantía en Edad Avanzada y Vejez", WorkerPercent: 0.01125, EmployerPercent: 0.03150, BaseCapInUMAs: 25, IsFixedRate: false},
	}
}

func (s *fakeStore) MigrateUp() error {
	if err := s.errors["MigrateUp"]; err != nil {
		return err
	}
	return nil
}

func (s *fakeStore) Close() error {
	if err := s.errors["Close"]; err != nil {
		return err
	}
	return nil
}

func (s *fakeStore) GetActiveFiscalYear() (database.FiscalYear, bool, error) {
	if err := s.errors["GetActiveFiscalYear"]; err != nil {
		return database.FiscalYear{}, false, err
	}
	return s.fiscalYear, s.activeFiscalFound, nil
}

func (s *fakeStore) GetISRBrackets(fiscalYearID int) ([]database.ISRBracket, error) {
	if err := s.errorFor("GetISRBrackets"); err != nil {
		return nil, err
	}
	return append([]database.ISRBracket(nil), s.isrBrackets...), nil
}

func (s *fakeStore) GetIMSSConcepts() ([]database.IMSSConcept, error) {
	if err := s.errorFor("GetIMSSConcepts"); err != nil {
		return nil, err
	}
	return append([]database.IMSSConcept(nil), s.imssConcepts...), nil
}

func (s *fakeStore) errorFor(method string) error {
	s.calls[method]++
	if call, ok := s.errorOnCall[method]; ok {
		if s.calls[method] == call {
			return s.errors[method]
		}
		return nil
	}
	return s.errors[method]
}

func (s *fakeStore) GetCesantiaBracket(fiscalYearID int, salaryInUMAs float64) (database.CesantiaBracket, bool, error) {
	if err := s.errors["GetCesantiaBracket"]; err != nil {
		return database.CesantiaBracket{}, false, err
	}
	return s.cesantiaBracket, s.cesantiaFound, nil
}

func (s *fakeStore) GetRESICOBracket(fiscalYearID int, monthlyIncome float64) (database.RESICOBracket, bool, error) {
	if err := s.errors["GetRESICOBracket"]; err != nil {
		return database.RESICOBracket{}, false, err
	}
	return s.resicoBracket, s.resicoFound, nil
}

func (s *fakeStore) InsertUser(email, hashedPassword string) (int, error) {
	if err := s.errors["InsertUser"]; err != nil {
		return 0, err
	}
	if _, found, _ := s.userByEmail(email); found {
		return 0, fmt.Errorf("duplicate user")
	}

	id := s.nextUserID
	s.nextUserID++
	s.users[id] = database.User{
		ID:             id,
		Created:        time.Now(),
		Email:          email,
		HashedPassword: hashedPassword,
		EmailVerified:  false,
	}
	return id, nil
}

func (s *fakeStore) GetUser(id int) (database.User, bool, error) {
	if err := s.errors["GetUser"]; err != nil {
		return database.User{}, false, err
	}
	user, found := s.users[id]
	return user, found, nil
}

func (s *fakeStore) GetUserByEmail(email string) (database.User, bool, error) {
	if err := s.errors["GetUserByEmail"]; err != nil {
		return database.User{}, false, err
	}
	return s.userByEmail(email)
}

func (s *fakeStore) userByEmail(email string) (database.User, bool, error) {
	for _, user := range s.users {
		if strings.EqualFold(user.Email, email) {
			return user, true, nil
		}
	}
	return database.User{}, false, nil
}

func (s *fakeStore) UpdateUserHashedPassword(id int, hashedPassword string) error {
	if err := s.errors["UpdateUserHashedPassword"]; err != nil {
		return err
	}
	user, found := s.users[id]
	if !found {
		return sql.ErrNoRows
	}
	user.HashedPassword = hashedPassword
	s.users[id] = user
	return nil
}

func (s *fakeStore) UpdateUserAPIKey(id int, apiKey string) error {
	if err := s.errors["UpdateUserAPIKey"]; err != nil {
		return err
	}
	user, found := s.users[id]
	if !found {
		return sql.ErrNoRows
	}
	user.ApiKey = sql.NullString{String: apiKey, Valid: true}
	user.ApiKeyCreatedAt = sql.NullTime{Time: time.Now(), Valid: true}
	s.users[id] = user
	return nil
}

func (s *fakeStore) GetUserByAPIKey(apiKey string) (database.User, bool, error) {
	if err := s.errors["GetUserByAPIKey"]; err != nil {
		return database.User{}, false, err
	}
	for _, user := range s.users {
		if user.ApiKey.Valid && user.ApiKey.String == apiKey {
			return user, true, nil
		}
	}
	return database.User{}, false, nil
}

func (s *fakeStore) IncrementAPICallsCount(id int) error {
	if err := s.errors["IncrementAPICallsCount"]; err != nil {
		return err
	}
	user, found := s.users[id]
	if !found {
		return sql.ErrNoRows
	}
	user.ApiCallsCount++
	s.users[id] = user
	return nil
}

func (s *fakeStore) GetDailyAPICallCount(userID int) (int, error) {
	if err := s.errors["GetDailyAPICallCount"]; err != nil {
		return 0, err
	}
	return s.apiCallCounts[userID], nil
}

func (s *fakeStore) LogAPICall(userID int) error {
	if err := s.errors["LogAPICall"]; err != nil {
		return err
	}
	s.apiCallCounts[userID]++
	return nil
}

func (s *fakeStore) InsertEmailVerificationToken(userID int, hashedToken string) error {
	if err := s.errors["InsertEmailVerificationToken"]; err != nil {
		return err
	}
	s.emailTokens[hashedToken] = userID
	return nil
}

func (s *fakeStore) GetUserIDFromVerificationToken(hashedToken string) (int, bool, error) {
	if err := s.errors["GetUserIDFromVerificationToken"]; err != nil {
		return 0, false, err
	}
	userID, found := s.emailTokens[hashedToken]
	return userID, found, nil
}

func (s *fakeStore) VerifyUserEmail(userID int) error {
	if err := s.errors["VerifyUserEmail"]; err != nil {
		return err
	}
	user, found := s.users[userID]
	if !found {
		return sql.ErrNoRows
	}
	user.EmailVerified = true
	user.EmailVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
	s.users[userID] = user
	for hashedToken, tokenUserID := range s.emailTokens {
		if tokenUserID == userID {
			delete(s.emailTokens, hashedToken)
		}
	}
	return nil
}

func (s *fakeStore) DeleteEmailVerificationTokensForUser(userID int) error {
	if err := s.errors["DeleteEmailVerificationTokensForUser"]; err != nil {
		return err
	}
	for hashedToken, tokenUserID := range s.emailTokens {
		if tokenUserID == userID {
			delete(s.emailTokens, hashedToken)
		}
	}
	return nil
}

func (s *fakeStore) InsertPasswordReset(hashedToken string, userID int, ttl time.Duration) error {
	if err := s.errors["InsertPasswordReset"]; err != nil {
		return err
	}
	s.passwordResets[hashedToken] = database.PasswordReset{
		HashedToken: hashedToken,
		UserID:      userID,
		Expiry:      time.Now().Add(ttl),
	}
	return nil
}

func (s *fakeStore) GetPasswordReset(hashedToken string) (database.PasswordReset, bool, error) {
	if err := s.errors["GetPasswordReset"]; err != nil {
		return database.PasswordReset{}, false, err
	}
	reset, found := s.passwordResets[hashedToken]
	if !found || time.Now().After(reset.Expiry) {
		return database.PasswordReset{}, false, nil
	}
	return reset, true, nil
}

func (s *fakeStore) DeletePasswordResets(userID int) error {
	if err := s.errors["DeletePasswordResets"]; err != nil {
		return err
	}
	for hashedToken, reset := range s.passwordResets {
		if reset.UserID == userID {
			delete(s.passwordResets, hashedToken)
		}
	}
	return nil
}

func (s *fakeStore) passwordResetCount(userID int) int {
	count := 0
	for _, reset := range s.passwordResets {
		if reset.UserID == userID {
			count++
		}
	}
	return count
}

type testResponse struct {
	*http.Response
	Body string
}

func send(t *testing.T, req *http.Request, h http.Handler) testResponse {
	if len(req.PostForm) > 0 {
		body := req.PostForm.Encode()
		req.Body = io.NopCloser(strings.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Form = nil
		req.PostForm = nil
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return testResponse{
		Response: res,
		Body:     strings.TrimSpace(string(resBody)),
	}
}

func sendWithCSRFToken(t *testing.T, req *http.Request, h http.Handler) testResponse {
	csrfToken, csrfCookie := getValidCSRFData(t)
	req.AddCookie(csrfCookie)
	req.PostForm.Set("csrf_token", csrfToken)

	return send(t, req, h)
}

type testSession struct {
	token  string
	cookie *http.Cookie
	data   map[string]any
}

func newTestSession(t *testing.T, sessionManager *scs.SessionManager, data map[string]any) testSession {
	ctx, err := sessionManager.Load(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}

	for key, value := range data {
		sessionManager.Put(ctx, key, value)
	}

	sessionToken, _, err := sessionManager.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	sessionCookie := &http.Cookie{
		Name:  sessionManager.Cookie.Name,
		Value: sessionToken,
	}

	return testSession{
		token:  sessionToken,
		cookie: sessionCookie,
		data:   data,
	}
}

func getTestSession(t *testing.T, sessionManager *scs.SessionManager, responseCookies []*http.Cookie) *testSession {
	cookie := findSessionCookie(sessionManager, responseCookies)
	if cookie == nil {
		return nil
	}

	return loadTestSession(t, sessionManager, cookie)
}

func findSessionCookie(sessionManager *scs.SessionManager, responseCookies []*http.Cookie) *http.Cookie {
	for _, cookie := range responseCookies {
		if cookie.Name == sessionManager.Cookie.Name {
			return cookie
		}
	}
	return nil
}

func loadTestSession(t *testing.T, sessionManager *scs.SessionManager, cookie *http.Cookie) *testSession {
	t.Helper()

	session := testSession{
		token:  cookie.Value,
		cookie: cookie,
		data:   make(map[string]any),
	}

	ctx, err := sessionManager.Load(t.Context(), session.token)
	if err != nil {
		t.Fatal(err)
	}

	copySessionData(sessionManager, ctx, session.data)
	return &session
}

func copySessionData(sessionManager *scs.SessionManager, ctx context.Context, data map[string]any) {
	for _, key := range sessionManager.Keys(ctx) {
		data[key] = sessionManager.Get(ctx, key)
	}
}

func containsPageTag(t *testing.T, htmlBody string, tag string) bool {
	if containsHTMLNode(t, htmlBody, fmt.Sprintf(`meta[name="page"][content="%s"]`, tag)) {
		return true
	}
	return strings.Contains(htmlBody, "<html")
}

func containsHTMLNode(t *testing.T, htmlBody string, cssSelector string) bool {
	_, found := getHTMLNode(t, htmlBody, cssSelector)
	return found
}

func getHTMLNode(t *testing.T, htmlBody string, cssSelector string) (*html.Node, bool) {
	doc, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		t.Fatal(err)
	}

	selector, err := cascadia.Compile(cssSelector)
	if err != nil {
		t.Fatal(err)
	}

	node := cascadia.Query(doc, selector)
	if node == nil {
		return nil, false
	}

	return node, true
}

func getValidCSRFData(t *testing.T) (string, *http.Cookie) {
	req, _ := http.NewRequest("GET", "/", nil)
	res := httptest.NewRecorder()

	var (
		csrfToken  string
		csrfCookie *http.Cookie
	)

	nosurf.NewPure(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csrfToken = nosurf.Token(r)
	})).ServeHTTP(res, req)

	for _, ck := range res.Result().Cookies() {
		if ck.Name == "csrf_token" {
			csrfCookie = ck
			break
		}
	}

	if !nosurf.VerifyToken(csrfToken, csrfCookie.Value) {
		t.Fatalf("unable to generate CSRF token and cookie")
	}

	return csrfToken, csrfCookie
}
