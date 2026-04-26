package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestBanxicoClient(t *testing.T) {
	client := NewBanxicoClient("token", discardLogger())
	client.httpClient = httpClient(http.StatusOK, `{"bmx":{"series":[{"datos":[{"fecha":"2026-04-25","dato":"18.42"}]}]}}`, nil)

	rate, err := client.GetExchangeRate(context.Background())

	assertNoError(t, err)
	assertFloat(t, rate, 18.42)
	assertString(t, client.ConfiguredToken(), "token")
}

func TestBanxicoRequest(t *testing.T) {
	client := NewBanxicoClient("token", discardLogger())

	req, err := client.newExchangeRateRequest(context.Background())

	assertNoError(t, err)
	assertString(t, req.Header.Get("Bmx-Token"), "token")
	assertString(t, req.Header.Get("Accept"), "application/json")
}

func TestBanxicoRequestError(t *testing.T) {
	client := NewBanxicoClient("token", discardLogger())
	withBanxicoAPIEndpoint(t, ":")

	_, err := client.newExchangeRateRequest(context.Background())

	assertError(t, err)
}

func TestBanxicoGetExchangeRateErrors(t *testing.T) {
	t.Run("returns fetch error", func(t *testing.T) {
		client := NewBanxicoClient("token", discardLogger())
		client.httpClient = httpClient(0, "", errors.New("network failed"))

		_, err := client.GetExchangeRate(context.Background())
		assertError(t, err)
	})

	t.Run("returns parse error", func(t *testing.T) {
		client := NewBanxicoClient("token", discardLogger())
		client.httpClient = httpClient(http.StatusOK, "{", nil)

		_, err := client.GetExchangeRate(context.Background())
		assertError(t, err)
	})

	t.Run("returns missing rate error", func(t *testing.T) {
		client := NewBanxicoClient("token", discardLogger())
		client.httpClient = httpClient(http.StatusOK, `{"bmx":{"series":[]}}`, nil)

		_, err := client.GetExchangeRate(context.Background())
		assertError(t, err)
	})
}

func TestBanxicoFetchErrors(t *testing.T) {
	t.Run("returns transport error", func(t *testing.T) {
		client := NewBanxicoClient("token", discardLogger())
		client.httpClient = httpClient(0, "", errors.New("network failed"))

		_, err := client.fetchExchangeRateResponse(context.Background())
		assertError(t, err)
	})

	t.Run("returns non-OK API error", func(t *testing.T) {
		client := NewBanxicoClient("token", discardLogger())
		client.httpClient = httpClient(http.StatusBadGateway, "bad gateway", nil)

		_, err := client.fetchExchangeRateResponse(context.Background())
		assertError(t, err)
	})

	t.Run("returns request creation error", func(t *testing.T) {
		client := NewBanxicoClient("token", discardLogger())
		withBanxicoAPIEndpoint(t, ":")

		_, err := client.fetchExchangeRateResponse(context.Background())
		assertError(t, err)
	})
}

func TestBanxicoParsingErrors(t *testing.T) {
	assertParseBanxicoError(t, "{")
	assertRateError(t, `{"bmx":{"series":[]}}`)
	assertRateError(t, `{"bmx":{"series":[{"datos":[]}]}}`)
	assertParseExchangeRateError(t, "bad")
	assertParseExchangeRateError(t, "9.99")
	assertParseExchangeRateError(t, "30.01")
}

func assertRateError(t *testing.T, body string) {
	t.Helper()

	data, err := parseBanxicoResponse([]byte(body))
	assertNoError(t, err)
	_, _, err = banxicoRateFromResponse(data)
	assertError(t, err)
}

func assertParseBanxicoError(t *testing.T, body string) {
	t.Helper()

	_, err := parseBanxicoResponse([]byte(body))
	assertError(t, err)
}

func assertParseExchangeRateError(t *testing.T, value string) {
	t.Helper()

	_, err := parseExchangeRate(value)
	assertError(t, err)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func httpClient(status int, body string, err error) *http.Client {
	return &http.Client{Transport: fakeTransport{status: status, body: body, err: err}}
}

type fakeTransport struct {
	status int
	body   string
	err    error
}

func (t fakeTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	if t.err != nil {
		return nil, t.err
	}

	return &http.Response{
		StatusCode: t.status,
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Header:     make(http.Header),
	}, nil
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func assertFloat(t *testing.T, got float64, want float64) {
	t.Helper()
	if got != want {
		t.Fatalf("got %f; want %f", got, want)
	}
}

func assertString(t *testing.T, got string, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}

func withBanxicoAPIEndpoint(t *testing.T, endpoint string) {
	t.Helper()

	previous := banxicoAPIEndpoint
	banxicoAPIEndpoint = endpoint
	t.Cleanup(func() {
		banxicoAPIEndpoint = previous
	})
}
