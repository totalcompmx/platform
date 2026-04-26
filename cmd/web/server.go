package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

const (
	defaultIdleTimeout    = time.Minute
	defaultReadTimeout    = 5 * time.Second
	defaultWriteTimeout   = 10 * time.Second
	defaultShutdownPeriod = 30 * time.Second
)

const (
	letsEncryptStagingCA    = "https://acme-staging-v02.api.letsencrypt.org/directory"
	letsEncryptProductionCA = "https://acme-v02.api.letsencrypt.org/directory"
)

var serveServer = func(app *application, srv *http.Server) error {
	return app.serve(srv)
}
var listenAndServeServer = listenAndServe
var shutdownOnSignalServer = shutdownOnSignal

func (app *application) serveHTTP() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.httpPort),
		Handler:      app.routes(),
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelWarn),
		IdleTimeout:  defaultIdleTimeout,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	err := serveServer(app, srv)
	if err != nil {
		return err
	}

	app.wg.Wait()
	return nil
}

func (app *application) serveAutoHTTPS() error {
	if err := validateAutoHTTPSDomain(app.config.autoHTTPS.domain); err != nil {
		return err
	}

	serverErrorChan := app.startAutoHTTPSServers(app.autoHTTPSCertManager())
	if err := firstServerError(serverErrorChan); err != nil {
		return err
	}

	app.wg.Wait()
	return nil
}

func validateAutoHTTPSDomain(domain string) error {
	if domain == "localhost" || strings.HasPrefix(domain, "localhost:") {
		return errors.New("auto HTTPS domain must be publicly accessible (not localhost)")
	}

	return nil
}

func (app *application) autoHTTPSCertManager() *autocert.Manager {
	return &autocert.Manager{
		Email:      app.config.autoHTTPS.email,
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache("./certs"),
		HostPolicy: autocert.HostWhitelist(app.config.autoHTTPS.domain),
		Client: &acme.Client{
			DirectoryURL: autoHTTPSDirectory(app.config.autoHTTPS.staging),
		},
	}
}

func autoHTTPSDirectory(staging bool) string {
	if staging {
		return letsEncryptStagingCA
	}

	return letsEncryptProductionCA
}

func (app *application) startAutoHTTPSServers(certManager *autocert.Manager) <-chan error {
	serverErrorChan := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go app.serveAutoHTTPSListener(certManager, &wg, serverErrorChan)
	go app.serveAutoHTTPRedirect(certManager, &wg, serverErrorChan)
	go closeWhenDone(&wg, serverErrorChan)

	return serverErrorChan
}

func (app *application) serveAutoHTTPSListener(certManager *autocert.Manager, wg *sync.WaitGroup, serverErrorChan chan<- error) {
	defer wg.Done()
	serverErrorChan <- serveServer(app, app.autoHTTPSServer(certManager))
}

func (app *application) autoHTTPSServer(certManager *autocert.Manager) *http.Server {
	tlsConfig := certManager.TLSConfig()
	tlsConfig.MinVersion = tls.VersionTLS12
	tlsConfig.CurvePreferences = []tls.CurveID{tls.X25519, tls.CurveP256}

	return &http.Server{
		Addr:         ":443",
		Handler:      app.routes(),
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelWarn),
		IdleTimeout:  defaultIdleTimeout,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		TLSConfig:    tlsConfig,
	}
}

func (app *application) serveAutoHTTPRedirect(certManager *autocert.Manager, wg *sync.WaitGroup, serverErrorChan chan<- error) {
	defer wg.Done()
	serverErrorChan <- serveServer(app, app.autoHTTPRedirectServer(certManager))
}

func (app *application) autoHTTPRedirectServer(certManager *autocert.Manager) *http.Server {
	return &http.Server{
		Addr:         ":80",
		Handler:      certManager.HTTPHandler(nil),
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelWarn),
		IdleTimeout:  defaultIdleTimeout,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}
}

func closeWhenDone(wg *sync.WaitGroup, ch chan<- error) {
	wg.Wait()
	close(ch)
}

func firstServerError(serverErrorChan <-chan error) error {
	for err := range serverErrorChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (app *application) serve(srv *http.Server) error {
	shutdownErrorChan := shutdownOnSignalServer(srv)

	app.logger.Info("starting server", slog.Group("server", "addr", srv.Addr))

	if err := listenAndServeServer(srv); err != nil {
		return err
	}

	if err := <-shutdownErrorChan; err != nil {
		return err
	}

	app.logger.Info("stopped server", slog.Group("server", "addr", srv.Addr))

	return nil
}

func shutdownOnSignal(srv *http.Server) <-chan error {
	shutdownErrorChan := make(chan error)

	go func() {
		quitChan := make(chan os.Signal, 1)
		signal.Notify(quitChan, syscall.SIGINT, syscall.SIGTERM)
		<-quitChan

		ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownPeriod)
		defer cancel()

		shutdownErrorChan <- srv.Shutdown(ctx)
	}()

	return shutdownErrorChan
}

func listenAndServe(srv *http.Server) error {
	return ignoreServerClosed(startServer(srv))
}

func startServer(srv *http.Server) error {
	if srv.TLSConfig != nil {
		return srv.ListenAndServeTLS("", "")
	}

	return srv.ListenAndServe()
}

func ignoreServerClosed(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
