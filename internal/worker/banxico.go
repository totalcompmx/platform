package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	banxicoAPIURL = "https://www.banxico.org.mx/SieAPIRest/service/v1/series/SF43718/datos/oportuno"
)

var banxicoAPIEndpoint = banxicoAPIURL

// BanxicoClient handles communication with Banxico API
type BanxicoClient struct {
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

// BanxicoResponse represents the API response structure
type BanxicoResponse struct {
	BMX struct {
		Series []struct {
			Datos []struct {
				Fecha string `json:"fecha"`
				Dato  string `json:"dato"`
			} `json:"datos"`
		} `json:"series"`
	} `json:"bmx"`
}

// NewBanxicoClient creates a new Banxico API client
func NewBanxicoClient(token string, logger *slog.Logger) *BanxicoClient {
	if token == "" {
		logger.Warn("BANXICO_TOKEN environment variable not set - exchange rate updates will fail")
	}

	return &BanxicoClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

func (c *BanxicoClient) ConfiguredToken() string {
	return c.token
}

// GetExchangeRate fetches the latest USD/MXN exchange rate from Banxico
func (c *BanxicoClient) GetExchangeRate(ctx context.Context) (float64, error) {
	body, err := c.fetchExchangeRateResponse(ctx)
	if err != nil {
		return 0, err
	}
	data, err := parseBanxicoResponse(body)
	if err != nil {
		return 0, err
	}
	rate, date, err := banxicoRateFromResponse(data)
	if err != nil {
		return 0, err
	}

	c.logger.Info("exchange rate fetched",
		"rate", rate,
		"date", date,
		"source", "banxico")

	return rate, nil
}

func (c *BanxicoClient) fetchExchangeRateResponse(ctx context.Context) ([]byte, error) {
	req, err := c.newExchangeRateRequest(ctx)
	if err != nil {
		return nil, err
	}
	return fetchClientResponse(c.httpClient, c.logger, req, "fetching exchange rate from banxico", "banxico")
}

func (c *BanxicoClient) newExchangeRateRequest(ctx context.Context) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", banxicoAPIEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Bmx-Token", c.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func parseBanxicoResponse(body []byte) (BanxicoResponse, error) {
	var data BanxicoResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return data, fmt.Errorf("failed to parse banxico response: %w", err)
	}
	return data, nil
}

func banxicoRateFromResponse(data BanxicoResponse) (float64, string, error) {
	if len(data.BMX.Series) == 0 {
		return 0, "", fmt.Errorf("no series data in banxico response")
	}
	return banxicoRateFromSeries(data.BMX.Series[0])
}

func banxicoRateFromSeries(series struct {
	Datos []struct {
		Fecha string `json:"fecha"`
		Dato  string `json:"dato"`
	} `json:"datos"`
}) (float64, string, error) {
	if len(series.Datos) == 0 {
		return 0, "", fmt.Errorf("no data points in banxico series")
	}
	rate, err := parseExchangeRate(series.Datos[0].Dato)
	return rate, series.Datos[0].Fecha, err
}

func parseExchangeRate(value string) (float64, error) {
	rate, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse exchange rate '%s': %w", value, err)
	}
	if rate < 10.0 || rate > 30.0 {
		return 0, fmt.Errorf("exchange rate %f is outside reasonable bounds", rate)
	}
	return rate, nil
}
