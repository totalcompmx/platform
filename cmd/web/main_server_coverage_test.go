package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"database/sql/driver"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
	"github.com/jcroyoaun/totalcompmx/internal/database"

	"github.com/jmoiron/sqlx"
)

func TestMainFunctionPaths(t *testing.T) {
	t.Run("runs requested task", func(t *testing.T) {
		store := newFakeStore(t)
		exitCode := -1
		restore := stubMainRuntime(t, []string{"web", "-task=migrate"}, store, nil, &exitCode)
		defer restore()

		main()

		assert.Equal(t, exitCode, -1)
	})

	t.Run("prints version and exits zero", func(t *testing.T) {
		store := newFakeStore(t)
		exitCode := -1
		restore := stubMainRuntime(t, []string{"web", "-version"}, store, nil, &exitCode)
		defer restore()

		main()

		assert.Equal(t, exitCode, 0)
	})

	t.Run("exits non-zero on run error", func(t *testing.T) {
		exitCode := -1
		restore := stubMainRuntime(t, []string{"web", "-task=migrate"}, nil, errors.New("open failed"), &exitCode)
		defer restore()

		main()

		assert.Equal(t, exitCode, 1)
	})
}

func TestRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("returns open database error", func(t *testing.T) {
		restore := stubRunRuntime(t, nil, errors.New("open failed"))
		defer restore()

		err := run(logger, config{}, "migrate")

		assertErrorContains(t, err, "open failed")
	})

	t.Run("runs app task and stops monitor", func(t *testing.T) {
		store := newFakeStore(t)
		restore := stubRunRuntime(t, store, nil)
		defer restore()

		err := run(logger, config{}, "migrate")

		assert.Nil(t, err)
	})
}

func TestApplicationConstructionAndTasks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("constructs production dependencies", func(t *testing.T) {
		db := newStubDatabase(t)
		defer db.Close()
		cfg := config{}
		cfg.session.cookieName = "session_test"
		cfg.cookie.secure = true
		cfg.resend.from = "sender@example.com"

		app := newApplication(logger, cfg, db)

		assert.Equal(t, app.config.session.cookieName, "session_test")
		assert.Equal(t, app.sessionManager.Cookie.Name, "session_test")
		assert.True(t, app.sessionManager.Cookie.Secure)
		assert.NotNil(t, app.mailer)
		assert.NotNil(t, app.worker)
	})

	t.Run("production app builder accepts database handle", func(t *testing.T) {
		db := newStubDatabase(t)
		defer db.Close()

		app := buildApplication(logger, config{}, db)

		assert.NotNil(t, app)
	})

	t.Run("production database opener reports bad DSN", func(t *testing.T) {
		_, err := openDatabase("%")

		assert.NotNil(t, err)
	})

	t.Run("builds real mailer when API key is configured", func(t *testing.T) {
		cfg := config{}
		cfg.resend.apiKey = "test-key"
		cfg.resend.from = "sender@example.com"

		assert.NotNil(t, newMailer(logger, cfg))
	})

	t.Run("rejects unknown task", func(t *testing.T) {
		app := newTestApplication(t)

		err := app.runTask("missing")

		assertErrorContains(t, err, "unknown task")
	})
}

func TestRunServerTask(t *testing.T) {
	t.Run("runs migration before HTTP server", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.db.automigrate = true
		app.config.httpPort = 19090
		var servedAddr string
		restore := stubServeServer(func(app *application, srv *http.Server) error {
			servedAddr = srv.Addr
			return nil
		})
		defer restore()

		err := app.runServerTask()

		assert.Nil(t, err)
		assert.Equal(t, servedAddr, ":19090")
	})

	t.Run("returns migration error", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.db.automigrate = true
		app.db.(*fakeStore).errors["MigrateUp"] = errors.New("migrate failed")

		err := app.runServerTask()

		assertErrorContains(t, err, "migrate failed")
	})

	t.Run("runs auto HTTPS servers", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.autoHTTPS.domain = "example.com"
		app.config.autoHTTPS.email = "admin@example.com"
		var served atomic.Int32
		restore := stubServeServer(func(app *application, srv *http.Server) error {
			served.Add(1)
			return nil
		})
		defer restore()

		err := app.runServerTask()

		assert.Nil(t, err)
		assert.Equal(t, served.Load(), int32(2))
	})

	t.Run("returns auto HTTPS server error", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.autoHTTPS.domain = "example.com"
		var wg sync.WaitGroup
		wg.Add(2)
		restore := stubServeServer(func(app *application, srv *http.Server) error {
			defer wg.Done()
			return errors.New("serve failed")
		})
		defer func() {
			wg.Wait()
			restore()
		}()

		err := app.serveAutoHTTPS()

		assertErrorContains(t, err, "serve failed")
	})
}

func TestServerHelpers(t *testing.T) {
	app := newTestApplication(t)
	app.config.autoHTTPS.domain = "example.com"
	app.config.autoHTTPS.email = "admin@example.com"

	assertErrorContains(t, validateAutoHTTPSDomain("localhost"), "publicly accessible")
	assertErrorContains(t, validateAutoHTTPSDomain("localhost:443"), "publicly accessible")
	assert.Nil(t, validateAutoHTTPSDomain("example.com"))
	assert.Equal(t, autoHTTPSDirectory(true), letsEncryptStagingCA)
	assert.Equal(t, autoHTTPSDirectory(false), letsEncryptProductionCA)

	certManager := app.autoHTTPSCertManager()
	assert.NotNil(t, certManager)
	assert.Equal(t, app.autoHTTPSServer(certManager).Addr, ":443")
	assert.Equal(t, app.autoHTTPRedirectServer(certManager).Addr, ":80")
}

func TestServe(t *testing.T) {
	app := newTestApplication(t)
	srv := &http.Server{Addr: ":0"}

	t.Run("returns listener error", func(t *testing.T) {
		restore := stubServeRuntime(
			func(*http.Server) <-chan error {
				ch := make(chan error, 1)
				ch <- nil
				return ch
			},
			func(*http.Server) error {
				return errors.New("listen failed")
			},
		)
		defer restore()

		err := app.serve(srv)

		assertErrorContains(t, err, "listen failed")
	})

	t.Run("returns shutdown error", func(t *testing.T) {
		restore := stubServeRuntime(
			func(*http.Server) <-chan error {
				ch := make(chan error, 1)
				ch <- errors.New("shutdown failed")
				return ch
			},
			func(*http.Server) error {
				return nil
			},
		)
		defer restore()

		err := app.serve(srv)

		assertErrorContains(t, err, "shutdown failed")
	})

	t.Run("returns nil on clean shutdown", func(t *testing.T) {
		restore := stubServeRuntime(
			func(*http.Server) <-chan error {
				ch := make(chan error, 1)
				ch <- nil
				return ch
			},
			func(*http.Server) error {
				return nil
			},
		)
		defer restore()

		err := app.serve(srv)

		assert.Nil(t, err)
	})
}

func TestLowLevelServerHelpers(t *testing.T) {
	assert.Nil(t, ignoreServerClosed(http.ErrServerClosed))
	assertErrorContains(t, ignoreServerClosed(errors.New("boom")), "boom")

	err := startServer(&http.Server{Addr: "bad-address"})
	assert.NotNil(t, err)
	err = startServer(&http.Server{Addr: "bad-address", TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}})
	assert.NotNil(t, err)

	ch := make(chan error, 2)
	ch <- nil
	ch <- errors.New("server failed")
	close(ch)
	assertErrorContains(t, firstServerError(ch), "server failed")

	empty := make(chan error)
	close(empty)
	assert.Nil(t, firstServerError(empty))
}

func stubMainRuntime(t *testing.T, args []string, store runDatabase, openErr error, exitCode *int) func() {
	t.Helper()
	restoreRun := stubRunRuntime(t, store, openErr)
	originalExit := exitProcess
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	os.Args = args
	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	exitProcess = func(code int) {
		*exitCode = code
	}
	return func() {
		exitProcess = originalExit
		os.Args = originalArgs
		flag.CommandLine = originalFlags
		restoreRun()
	}
}

func stubRunRuntime(t *testing.T, store runDatabase, openErr error) func() {
	t.Helper()
	originalOpen := openDatabase
	originalBuild := buildApplication
	openDatabase = func(dsn string) (runDatabase, error) {
		if openErr != nil {
			return nil, openErr
		}
		return store, nil
	}
	buildApplication = func(logger *slog.Logger, cfg config, db runDatabase) *application {
		app := newTestApplication(t)
		app.config = cfg
		app.db = db
		return app
	}
	return func() {
		openDatabase = originalOpen
		buildApplication = originalBuild
	}
}

func stubServeServer(fn func(*application, *http.Server) error) func() {
	original := serveServer
	serveServer = fn
	return func() {
		serveServer = original
	}
}

func stubServeRuntime(shutdown func(*http.Server) <-chan error, listen func(*http.Server) error) func() {
	originalShutdown := shutdownOnSignalServer
	originalListen := listenAndServeServer
	shutdownOnSignalServer = shutdown
	listenAndServeServer = listen
	return func() {
		shutdownOnSignalServer = originalShutdown
		listenAndServeServer = originalListen
	}
}

func newStubDatabase(t *testing.T) *database.DB {
	t.Helper()
	sqlDB := sql.OpenDB(stubConnector{})
	t.Cleanup(func() {
		sqlDB.Close()
	})
	return &database.DB{DB: sqlx.NewDb(sqlDB, "postgres")}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q; got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("got error %q; want to contain %q", err, want)
	}
}

type stubConnector struct{}

func (stubConnector) Connect(context.Context) (driver.Conn, error) {
	return stubConn{}, nil
}

func (stubConnector) Driver() driver.Driver {
	return stubDriver{}
}

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) {
	return stubConn{}, nil
}

type stubConn struct{}

func (stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (stubConn) Close() error {
	return nil
}

func (stubConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}
