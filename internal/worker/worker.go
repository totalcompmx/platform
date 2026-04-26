package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/database"
)

// Worker manages background ETL jobs for financial data updates
type Worker struct {
	db     workerDB
	logger *slog.Logger

	// Clients
	banxico exchangeRateClient
	inegi   umaClient

	// Control
	stopChan chan struct{}
}

type workerDB interface {
	UpdateExchangeRate(float64) error
	UpsertUMAForYear(int, float64, float64, float64) error
}

type exchangeRateClient interface {
	GetExchangeRate(context.Context) (float64, error)
}

type umaClient interface {
	GetUMA(context.Context) (*UMAData, error)
}

type tokenClient interface {
	ConfiguredToken() string
}

// New creates a new Worker instance
func New(db *database.DB, logger *slog.Logger, banxicoToken, inegiToken string) *Worker {
	return &Worker{
		db:       db,
		logger:   logger,
		banxico:  NewBanxicoClient(banxicoToken, logger),
		inegi:    NewINEGIClient(inegiToken, logger),
		stopChan: make(chan struct{}),
	}
}

// Start begins the worker's job scheduling
func (w *Worker) Start() {
	w.logger.Info("worker started", "service", "etl_worker")

	// Run initial jobs immediately on startup
	w.runInitialJobs()

	// Schedule Banxico (Daily at 14:00 CST)
	go w.scheduleBanxico()

	// Schedule INEGI (Weekly on Mondays at 09:00 CST)
	go w.scheduleINEGI()

	// Wait for stop signal
	<-w.stopChan
	w.logger.Info("worker stopped", "service", "etl_worker")
}

// Stop gracefully shuts down the worker
func (w *Worker) Stop() {
	close(w.stopChan)
}

// runInitialJobs executes all jobs once at startup
func (w *Worker) runInitialJobs() {
	w.logger.Info("running initial ETL jobs")
	w.runInitialJob("Banxico", configuredToken(w.banxico), w.updateExchangeRate)
	w.runInitialJob("INEGI", configuredToken(w.inegi), w.updateUMA)
}

func (w *Worker) runInitialJob(name string, token string, run func() error) {
	if token == "" {
		w.logger.Warn("skipping initial " + name + " update - no token configured")
		return
	}
	if err := run(); err != nil {
		w.logger.Error("initial "+name+" update failed", "error", err)
	}
}

// scheduleBanxico runs daily at 14:00 CST
func (w *Worker) scheduleBanxico() {
	if configuredToken(w.banxico) == "" {
		w.logger.Info("banxico scheduler disabled - no token configured")
		return
	}
	w.scheduleLoop(time.NewTicker(1*time.Hour), shouldRunBanxico, "banxico", w.updateExchangeRate)
}

// scheduleINEGI runs weekly on Mondays at 09:00 CST
func (w *Worker) scheduleINEGI() {
	if configuredToken(w.inegi) == "" {
		w.logger.Info("inegi scheduler disabled - no token configured")
		return
	}
	w.scheduleLoop(time.NewTicker(6*time.Hour), shouldRunINEGI, "inegi", w.updateUMA)
}

func (w *Worker) scheduleLoop(ticker *time.Ticker, shouldRun func(time.Time) bool, name string, run func() error) {
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.runScheduledJob(shouldRun, name, run)
		case <-w.stopChan:
			return
		}
	}
}

func (w *Worker) runScheduledJob(shouldRun func(time.Time) bool, name string, run func() error) {
	if !shouldRun(time.Now().In(cstLocation())) {
		return
	}
	w.logger.Info("running scheduled " + name + " update")
	if err := run(); err != nil {
		w.logger.Error(name+" update failed", "error", err)
	}
}

func shouldRunBanxico(now time.Time) bool {
	return now.Hour() == 14 && now.Minute() < 60
}

func shouldRunINEGI(now time.Time) bool {
	return now.Weekday() == time.Monday && now.Hour() == 9 && now.Minute() < 360
}

// FetchUSDMXN triggers an immediate update of the USD/MXN exchange rate
func (w *Worker) FetchUSDMXN() error {
	return w.updateExchangeRate()
}

// FetchUMA triggers an immediate update of UMA values
func (w *Worker) FetchUMA() error {
	return w.updateUMA()
}

// updateExchangeRate fetches and updates USD/MXN rate from Banxico
func (w *Worker) updateExchangeRate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rate, err := w.banxico.GetExchangeRate(ctx)
	if err != nil {
		return err
	}

	// Update the active fiscal year
	if err := w.db.UpdateExchangeRate(rate); err != nil {
		return err
	}

	w.logger.Info("exchange rate updated", "rate", rate, "source", "banxico")
	return nil
}

// updateUMA fetches and updates UMA from INEGI
func (w *Worker) updateUMA() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	umaData, err := w.inegi.GetUMA(ctx)
	if err != nil {
		return err
	}

	// Store UMA values on the source-year row and make that row active.
	if err := w.db.UpsertUMAForYear(umaData.Year, umaData.Annual, umaData.Monthly, umaData.Daily); err != nil {
		return err
	}

	w.logger.Info("uma updated and fiscal year activated",
		"year", umaData.Year,
		"annual", umaData.Annual,
		"monthly", umaData.Monthly,
		"daily", umaData.Daily,
		"source", "inegi")
	return nil
}

// cstLocation returns CST timezone
func cstLocation() *time.Location {
	loc, err := loadLocation("America/Mexico_City")
	if err != nil {
		// Fallback to UTC-6 (CST)
		return time.FixedZone("CST", -6*60*60)
	}
	return loc
}

var loadLocation = time.LoadLocation

func configuredToken(client any) string {
	tokenClient, ok := client.(tokenClient)
	if !ok {
		return ""
	}

	return tokenClient.ConfiguredToken()
}
