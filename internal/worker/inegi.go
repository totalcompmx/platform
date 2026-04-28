package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// INEGI API endpoint for UMA (Indicator 539262)
	// Returns JSONP, so we need to strip the callback wrapper
	inegiAPIURL = "https://www.inegi.org.mx/app/api/indicadores/interna_v1_3/ValorIndicador/539262/00/null/es/null/null/3/pd/0/null/null/null/null/6/json/%s"
)

var inegiAPIEndpoint = inegiAPIURL

// INEGIClient handles communication with INEGI API
type INEGIClient struct {
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

// INEGIResponse represents the API response structure
// INEGI returns JSONP format: callback({"value": [...], "dimension": {...}})
type INEGIResponse struct {
	Value     []string `json:"value"` // Array of UMA values, first is current year
	Dimension struct {
		Periods struct {
			Category struct {
				Label []struct {
					Key   string `json:"Key"`   // "P1", "P2", etc.
					Value string `json:"Value"` // "2025", "2024", etc.
				} `json:"label"`
			} `json:"category"`
		} `json:"periods"`
	} `json:"dimension"`
}

// UMAData represents the calculated UMA values
type UMAData struct {
	Year    int
	Annual  float64
	Monthly float64
	Daily   float64
}

// NewINEGIClient creates a new INEGI API client
func NewINEGIClient(token string, logger *slog.Logger) *INEGIClient {
	if token == "" {
		logger.Warn("INEGI_TOKEN environment variable not set - UMA updates will fail")
	}

	return &INEGIClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

func (c *INEGIClient) ConfiguredToken() string {
	return c.token
}

// GetUMA fetches the latest UMA value from INEGI
func (c *INEGIClient) GetUMA(ctx context.Context) (*UMAData, error) {
	body, err := c.fetchUMAResponse(ctx)
	if err != nil {
		return nil, err
	}
	data, err := parseINEGIResponse(body)
	if err != nil {
		return nil, err
	}
	return c.umaDataFromResponse(data)
}

func (c *INEGIClient) fetchUMAResponse(ctx context.Context) ([]byte, error) {
	req, err := c.newUMARequest(ctx)
	if err != nil {
		return nil, err
	}
	return fetchClientResponse(c.httpClient, c.logger, req, "fetching uma from inegi", "inegi")
}

func (c *INEGIClient) newUMARequest(ctx context.Context) (*http.Request, error) {
	url := fmt.Sprintf(inegiAPIEndpoint, c.token)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func readOKResponse(resp *http.Response, source string) ([]byte, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s api returned status %d: %s", source, resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return body, nil
}

func parseINEGIResponse(body []byte) (INEGIResponse, error) {
	jsonStr := stripJSONPCallback(string(body))
	var data INEGIResponse
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return data, fmt.Errorf("failed to parse inegi response: %w", err)
	}
	if err := validateINEGIResponse(data); err != nil {
		return data, err
	}
	return data, nil
}

func validateINEGIResponse(data INEGIResponse) error {
	if len(data.Value) == 0 {
		return fmt.Errorf("no values in inegi response")
	}
	if len(data.Dimension.Periods.Category.Label) == 0 {
		return fmt.Errorf("no period labels in inegi response")
	}
	return nil
}

func (c *INEGIClient) umaDataFromResponse(data INEGIResponse) (*UMAData, error) {
	annual, year, err := c.findUMAObservation(data, time.Now().Year())
	if err != nil {
		return nil, err
	}
	umaData, err := newUMAData(year, annual)
	if err != nil {
		return nil, err
	}
	c.logger.Info("uma fetched and calculated", "year", umaData.Year, "annual", umaData.Annual, "monthly", umaData.Monthly, "daily", umaData.Daily, "source", "inegi")
	return umaData, nil
}

func (c *INEGIClient) findUMAObservation(data INEGIResponse, currentYear int) (float64, int, error) {
	annual, found, err := c.umaForYear(data, currentYear)
	if err != nil {
		return 0, 0, err
	}
	if found {
		return annual, currentYear, nil
	}
	return c.latestUMAObservation(data)
}

func (c *INEGIClient) umaForYear(data INEGIResponse, year int) (float64, bool, error) {
	yearStr := strconv.Itoa(year)
	for i, period := range data.Dimension.Periods.Category.Label {
		if period.Value == yearStr {
			return c.parseUMAAt(data, i, year)
		}
	}
	return 0, false, nil
}

func (c *INEGIClient) parseUMAAt(data INEGIResponse, index int, year int) (float64, bool, error) {
	if index >= len(data.Value) || data.Value[index] == "NA" {
		return 0, false, nil
	}
	annual, err := parseUMAAnnual(data.Value[index])
	if err != nil {
		return 0, false, fmt.Errorf("failed to parse uma value '%s': %w", data.Value[index], err)
	}
	c.logger.Info("found uma for year", "year", year, "value", annual, "index", index)
	return annual, true, nil
}

func (c *INEGIClient) latestUMAObservation(data INEGIResponse) (float64, int, error) {
	if data.Value[0] == "NA" {
		return 0, 0, fmt.Errorf("no valid uma values found in inegi data")
	}
	annual, err := parseUMAAnnual(data.Value[0])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse latest uma value '%s': %w", data.Value[0], err)
	}
	year, err := strconv.Atoi(data.Dimension.Periods.Category.Label[0].Value)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse inegi period year '%s': %w", data.Dimension.Periods.Category.Label[0].Value, err)
	}
	c.logger.Info("using latest uma observation", "year", year, "value", annual)
	return annual, year, nil
}

func parseUMAAnnual(value string) (float64, error) {
	valueStr := strings.ReplaceAll(value, ",", "")
	return strconv.ParseFloat(valueStr, 64)
}

func newUMAData(year int, annual float64) (*UMAData, error) {
	if annual < 30000 || annual > 60000 {
		return nil, fmt.Errorf("uma annual %f is outside reasonable bounds (30k-60k)", annual)
	}
	return &UMAData{
		Year:    year,
		Annual:  annual,
		Monthly: annual / 12.0,
		Daily:   annual / 365.0,
	}, nil
}

// stripJSONPCallback removes the JSONP callback wrapper
// Input:  jQuery111209395642957513894_1762831315776({...})
// Output: {...}
func stripJSONPCallback(jsonp string) string {
	// Match pattern: callback({...})
	re := regexp.MustCompile(`^[a-zA-Z_$][a-zA-Z0-9_$]*\((.*)\);?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(jsonp))

	if len(matches) > 1 {
		return matches[1]
	}

	// If no callback found, return as-is (might be pure JSON)
	return jsonp
}
