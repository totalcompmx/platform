package main

import (
	"context"
	"encoding/gob"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/env"
	"github.com/jcroyoaun/totalcompmx/internal/smtp"
	"github.com/jcroyoaun/totalcompmx/internal/version"
	"github.com/jcroyoaun/totalcompmx/internal/worker"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/lmittmann/tint"
)

var exitProcess = os.Exit
var openDatabase = func(dsn string) (runDatabase, error) {
	return database.New(dsn)
}
var buildApplication = func(logger *slog.Logger, cfg config, db runDatabase) *application {
	return newApplication(logger, cfg, db.(*database.DB))
}
var sendMail = func(mailer *smtp.Mailer, recipient string, data any, patterns ...string) error {
	return mailer.Send(recipient, data, patterns...)
}
var renewSessionToken = func(sessionManager *scs.SessionManager, ctx context.Context) error {
	return sessionManager.RenewToken(ctx)
}

func init() {
	// Register types for gob encoding (used by session manager)
	gob.Register([]PackageInput{})
	gob.Register(PackageInput{})
	gob.Register([]PackageResult{})
	gob.Register(PackageResult{})
	gob.Register([]OtherBenefit{})
	gob.Register(OtherBenefit{})
	gob.Register(database.SalaryCalculation{})
	gob.Register(database.FiscalYear{})
	gob.Register([]database.OtherBenefitResult{})
	gob.Register(database.OtherBenefitResult{})
}

func main() {
	logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelDebug}))

	var cfg config

	// Config Loading (Env Vars)
	cfg.worker.banxicoToken = env.GetString("BANXICO_TOKEN", "")
	cfg.worker.inegiToken = env.GetString("INEGI_TOKEN", "")
	cfg.resend.apiKey = env.GetString("RESEND_API_KEY", "")
	cfg.cookie.secretKey = env.GetString("COOKIE_SECRET_KEY", "v2zn7or6otz36wqjt7b2qkj2xj3g7ug5")
	cfg.cookie.secure = env.GetBool("COOKIE_SECURE", true)

	dbUser := env.GetString("DB_USER", "totalcomp_app")
	dbPassword := env.GetString("DB_PASSWORD", "")
	dbHost := env.GetString("DB_HOST", "localhost")
	dbName := env.GetString("DB_NAME", "totalcomp")
	dbSSLMode := env.GetString("DB_SSLMODE", "require")
	// database.New prepends postgres:// so we construct user:pass@host/dbname
	cfg.db.dsn = fmt.Sprintf("%s:%s@%s/%s?sslmode=%s", dbUser, dbPassword, dbHost, dbName, dbSSLMode)

	cfg.baseURL = env.GetString("BASE_URL", "http://localhost:3080")
	cfg.httpPort = env.GetInt("HTTP_PORT", 3080)
	cfg.autoHTTPS.domain = env.GetString("AUTO_HTTPS_DOMAIN", "")
	cfg.autoHTTPS.email = env.GetString("AUTO_HTTPS_EMAIL", "admin@github.com/jcroyoaun/totalcompmx")
	cfg.autoHTTPS.staging = env.GetBool("AUTO_HTTPS_STAGING", false)
	cfg.basicAuth.username = env.GetString("BASIC_AUTH_USERNAME", "admin")
	cfg.basicAuth.hashedPassword = env.GetString("BASIC_AUTH_HASHED_PASSWORD", "$2a$10$jRb2qniNcoCyQM23T59RfeEQUbgdAXfR6S0scynmKfJa5Gj3arGJa")
	cfg.db.automigrate = env.GetBool("DB_AUTOMIGRATE", true)
	cfg.notifications.email = env.GetString("NOTIFICATIONS_EMAIL", "")
	cfg.session.cookieName = env.GetString("SESSION_COOKIE_NAME", "session_totalcomp")
	cfg.resend.from = env.GetString("RESEND_FROM", "TotalComp MX <hola@totalcomp.mx>")

	// CLI Switch
	task := flag.String("task", "server", "Task to execute (server, migrate, fetch-banxico, fetch-uma)")
	showVersion := flag.Bool("version", false, "display version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("version: %s\n", version.Get())
		exitProcess(0)
		return
	}

	// Execution Logic
	if err := run(logger, cfg, *task); err != nil {
		trace := string(debug.Stack())
		logger.Error(err.Error(), "trace", trace)
		exitProcess(1)
		return
	}
}

type config struct {
	baseURL   string
	httpPort  int
	basicAuth struct {
		username       string
		hashedPassword string
	}
	autoHTTPS struct {
		domain  string
		email   string
		staging bool
	}
	cookie struct {
		secretKey string
		secure    bool
	}
	db struct {
		dsn         string
		automigrate bool
	}
	notifications struct {
		email string
	}
	session struct {
		cookieName string
	}
	resend struct {
		apiKey string
		from   string
	}
	worker struct {
		banxicoToken string
		inegiToken   string
	}
}

type application struct {
	config         config
	db             dataStore
	logger         *slog.Logger
	mailer         *smtp.Mailer
	sessionManager *scs.SessionManager
	worker         *worker.Worker
	wg             sync.WaitGroup
}

type runDatabase interface {
	dataStore
	Close() error
}

func run(logger *slog.Logger, cfg config, task string) error {
	db, err := openDatabase(cfg.db.dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	app := buildApplication(logger, cfg, db)
	return app.runTask(task)
}

func newApplication(logger *slog.Logger, cfg config, db *database.DB) *application {
	return &application{
		config:         cfg,
		db:             db,
		logger:         logger,
		mailer:         newMailer(logger, cfg),
		sessionManager: newSessionManager(cfg, db),
		worker:         worker.New(db, logger, cfg.worker.banxicoToken, cfg.worker.inegiToken),
	}
}

func newMailer(logger *slog.Logger, cfg config) *smtp.Mailer {
	if cfg.resend.apiKey == "" {
		logger.Warn("RESEND_API_KEY not set, using mock mailer")
		return smtp.NewMockMailer(cfg.resend.from)
	}

	return smtp.NewMailer(cfg.resend.apiKey, cfg.resend.from)
}

func newSessionManager(cfg config, db *database.DB) *scs.SessionManager {
	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db.DB.DB)
	sessionManager.Lifetime = 7 * 24 * time.Hour
	sessionManager.Cookie.Name = cfg.session.cookieName
	sessionManager.Cookie.Secure = cfg.cookie.secure
	return sessionManager
}

type taskHandler func() error

func (app *application) runTask(task string) error {
	handler, ok := app.taskHandlers()[task]
	if !ok {
		return fmt.Errorf("unknown task: %s", task)
	}

	return handler()
}

func (app *application) taskHandlers() map[string]taskHandler {
	return map[string]taskHandler{
		"server":        app.runServerTask,
		"migrate":       app.db.MigrateUp,
		"fetch-banxico": app.worker.FetchUSDMXN,
		"fetch-uma":     app.worker.FetchUMA,
	}
}

func (app *application) runServerTask() error {
	if app.config.db.automigrate {
		if err := app.db.MigrateUp(); err != nil {
			return err
		}
	}

	if app.config.autoHTTPS.domain != "" {
		return app.serveAutoHTTPS()
	}

	return app.serveHTTP()
}
