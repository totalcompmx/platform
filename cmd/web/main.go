package main

import (
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
		os.Exit(0)
	}

	// Execution Logic
	if err := run(logger, cfg, *task); err != nil {
		trace := string(debug.Stack())
		logger.Error(err.Error(), "trace", trace)
		os.Exit(1)
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
	db             *database.DB
	logger         *slog.Logger
	mailer         *smtp.Mailer
	sessionManager *scs.SessionManager
	worker         *worker.Worker
	wg             sync.WaitGroup
}

func run(logger *slog.Logger, cfg config, task string) error {
	db, err := database.New(cfg.db.dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	
	// Start database connection pool monitoring
	db.MonitorConnectionPool()

	// Initialize mailer
	var mailer *smtp.Mailer
	if cfg.resend.apiKey == "" {
		logger.Warn("RESEND_API_KEY not set, using mock mailer")
		mailer = smtp.NewMockMailer(cfg.resend.from)
	} else {
		mailer = smtp.NewMailer(cfg.resend.apiKey, cfg.resend.from)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db.DB.DB)
	sessionManager.Lifetime = 7 * 24 * time.Hour
	sessionManager.Cookie.Name = cfg.session.cookieName
	sessionManager.Cookie.Secure = true

	etlWorker := worker.New(db, logger, cfg.worker.banxicoToken, cfg.worker.inegiToken)

	app := &application{
		config:         cfg,
		db:             db,
		logger:         logger,
		mailer:         mailer,
		sessionManager: sessionManager,
		worker:         etlWorker,
	}

	switch task {
	case "server":
		// Note: We do not start the background worker scheduler in server mode 
		// because we use CronJobs for those tasks now.
		
		// Run migrations if automigrate is enabled (or we could rely on init container)
		if cfg.db.automigrate {
			if err = db.MigrateUp(); err != nil {
				return err
			}
		}

		if cfg.autoHTTPS.domain != "" {
			return app.serveAutoHTTPS()
		}
		return app.serveHTTP()

	case "migrate":
		// Initialize DB connection done above
		if err := app.db.MigrateUp(); err != nil {
			return err
		}
		return nil

	case "fetch-banxico":
		if err := app.worker.FetchUSDMXN(); err != nil {
			return err
		}
		return nil

	case "fetch-uma":
		if err := app.worker.FetchUMA(); err != nil {
			return err
		}
		return nil

	default:
		return fmt.Errorf("unknown task: %s", task)
	}
}
