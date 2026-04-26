package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerNewStartStop(t *testing.T) {
	worker := New(nil, discardLogger(), "", "")
	done := make(chan struct{})

	go func() {
		worker.Start()
		close(done)
	}()

	worker.Stop()
	<-done
}

func TestWorkerUpdates(t *testing.T) {
	t.Run("updates exchange rate", func(t *testing.T) {
		worker, db := testWorker()

		err := worker.FetchUSDMXN()

		assertNoError(t, err)
		assertFloat(t, db.rate, 18.5)
	})

	t.Run("updates UMA", func(t *testing.T) {
		worker, db := testWorker()

		err := worker.FetchUMA()

		assertNoError(t, err)
		assertInt(t, db.year, 2026)
		assertFloat(t, db.annual, 41273.52)
	})
}

func TestWorkerUpdateErrors(t *testing.T) {
	t.Run("returns exchange rate client error", func(t *testing.T) {
		worker, _ := testWorker()
		worker.banxico = fakeRateClient{err: errors.New("rate failed")}

		assertError(t, worker.FetchUSDMXN())
	})

	t.Run("returns exchange rate DB error", func(t *testing.T) {
		worker, db := testWorker()
		db.err = errors.New("db failed")

		assertError(t, worker.FetchUSDMXN())
	})

	t.Run("returns UMA client error", func(t *testing.T) {
		worker, _ := testWorker()
		worker.inegi = fakeUMAClient{err: errors.New("uma failed")}

		assertError(t, worker.FetchUMA())
	})

	t.Run("returns UMA DB error", func(t *testing.T) {
		worker, db := testWorker()
		db.err = errors.New("db failed")

		assertError(t, worker.FetchUMA())
	})
}

func TestWorkerSchedulingHelpers(t *testing.T) {
	worker, _ := testWorker()
	var calls int32

	worker.runInitialJob("test", "", func() error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	worker.runInitialJob("test", "token", func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("failed")
	})
	worker.runScheduledJob(func(time.Time) bool { return false }, "test", func() error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	worker.runScheduledJob(func(time.Time) bool { return true }, "test", func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("failed")
	})

	assertInt(t, int(calls), 2)
}

func TestWorkerScheduleLoop(t *testing.T) {
	worker, _ := testWorker()
	done := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		worker.scheduleLoop(time.NewTicker(time.Nanosecond), func(time.Time) bool { return true }, "test", func() error {
			stopOnce.Do(worker.Stop)
			return nil
		})
		close(done)
	}()

	<-done
}

func TestWorkerSchedulersWithConfiguredTokens(t *testing.T) {
	t.Run("banxico exits on stop", func(t *testing.T) {
		worker, _ := testWorker()
		done := make(chan struct{})

		go func() {
			worker.scheduleBanxico()
			close(done)
		}()

		worker.Stop()
		<-done
	})

	t.Run("inegi exits on stop", func(t *testing.T) {
		worker, _ := testWorker()
		done := make(chan struct{})

		go func() {
			worker.scheduleINEGI()
			close(done)
		}()

		worker.Stop()
		<-done
	})
}

func TestShouldRunSchedules(t *testing.T) {
	assertBool(t, shouldRunBanxico(time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC)), true)
	assertBool(t, shouldRunBanxico(time.Date(2026, 1, 1, 13, 30, 0, 0, time.UTC)), false)
	assertBool(t, shouldRunINEGI(time.Date(2026, 1, 5, 9, 30, 0, 0, time.UTC)), true)
	assertBool(t, shouldRunINEGI(time.Date(2026, 1, 6, 9, 30, 0, 0, time.UTC)), false)
	assertString(t, configuredToken(struct{}{}), "")
	assertString(t, cstLocation().String(), "America/Mexico_City")
}

func TestCSTLocationFallback(t *testing.T) {
	previous := loadLocation
	loadLocation = func(string) (*time.Location, error) {
		return nil, errors.New("missing location")
	}
	t.Cleanup(func() {
		loadLocation = previous
	})

	assertString(t, cstLocation().String(), "CST")
}

func testWorker() (*Worker, *fakeWorkerDB) {
	db := &fakeWorkerDB{}
	return &Worker{
		db:       db,
		logger:   discardLogger(),
		banxico:  fakeRateClient{token: "rate-token", rate: 18.5},
		inegi:    fakeUMAClient{token: "uma-token", data: &UMAData{Year: 2026, Annual: 41273.52, Monthly: 3439.46, Daily: 113.08}},
		stopChan: make(chan struct{}),
	}, db
}

type fakeWorkerDB struct {
	rate   float64
	year   int
	annual float64
	err    error
}

func (db *fakeWorkerDB) UpdateExchangeRate(rate float64) error {
	db.rate = rate
	return db.err
}

func (db *fakeWorkerDB) UpsertUMAForYear(year int, annual, _, _ float64) error {
	db.year = year
	db.annual = annual
	return db.err
}

type fakeRateClient struct {
	token string
	rate  float64
	err   error
}

func (c fakeRateClient) ConfiguredToken() string {
	return c.token
}

func (c fakeRateClient) GetExchangeRate(context.Context) (float64, error) {
	return c.rate, c.err
}

type fakeUMAClient struct {
	token string
	data  *UMAData
	err   error
}

func (c fakeUMAClient) ConfiguredToken() string {
	return c.token
}

func (c fakeUMAClient) GetUMA(context.Context) (*UMAData, error) {
	return c.data, c.err
}
