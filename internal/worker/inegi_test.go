package worker

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestINEGIClient(t *testing.T) {
	body := fmt.Sprintf(`callback({"value":["41,273.52"],"dimension":{"periods":{"category":{"label":[{"Key":"P1","Value":"%d"}]}}}})`, time.Now().Year())
	client := NewINEGIClient("token", discardLogger())
	client.httpClient = httpClient(http.StatusOK, body, nil)

	uma, err := client.GetUMA(context.Background())

	assertNoError(t, err)
	assertInt(t, uma.Year, time.Now().Year())
	assertFloat(t, uma.Annual, 41273.52)
	assertString(t, client.ConfiguredToken(), "token")
}

func TestINEGIRequest(t *testing.T) {
	client := NewINEGIClient("token", discardLogger())

	req, err := client.newUMARequest(context.Background())

	assertNoError(t, err)
	assertString(t, req.Header.Get("Accept"), "application/json")
	assertBool(t, strings.Contains(req.URL.String(), "token"), true)
}

func TestINEGIRequestError(t *testing.T) {
	client := NewINEGIClient("token", discardLogger())
	withINEGIAPIEndpoint(t, "%%")

	_, err := client.newUMARequest(context.Background())

	assertError(t, err)
}

func TestINEGIGetUMAErrors(t *testing.T) {
	t.Run("returns fetch error", func(t *testing.T) {
		client := NewINEGIClient("token", discardLogger())
		client.httpClient = httpClient(0, "", context.Canceled)

		_, err := client.GetUMA(context.Background())
		assertError(t, err)
	})

	t.Run("returns parse error", func(t *testing.T) {
		client := NewINEGIClient("token", discardLogger())
		client.httpClient = httpClient(http.StatusOK, "{", nil)

		_, err := client.GetUMA(context.Background())
		assertError(t, err)
	})

	t.Run("returns UMA data error", func(t *testing.T) {
		body := fmt.Sprintf(`{"value":["1"],"dimension":{"periods":{"category":{"label":[{"Key":"P1","Value":"%d"}]}}}}`, time.Now().Year())
		client := NewINEGIClient("token", discardLogger())
		client.httpClient = httpClient(http.StatusOK, body, nil)

		_, err := client.GetUMA(context.Background())
		assertError(t, err)
	})
}

func TestINEGIFetchErrors(t *testing.T) {
	t.Run("returns transport error", func(t *testing.T) {
		client := NewINEGIClient("token", discardLogger())
		client.httpClient = httpClient(0, "", context.Canceled)

		_, err := client.fetchUMAResponse(context.Background())
		assertError(t, err)
	})

	t.Run("returns read error", func(t *testing.T) {
		_, err := readOKResponse(&http.Response{StatusCode: http.StatusOK, Body: errBody{}}, "inegi")
		assertError(t, err)
	})

	t.Run("returns request creation error", func(t *testing.T) {
		client := NewINEGIClient("token", discardLogger())
		withINEGIAPIEndpoint(t, "%%")

		_, err := client.fetchUMAResponse(context.Background())
		assertError(t, err)
	})
}

func TestINEGIParsingErrors(t *testing.T) {
	assertParseINEGIError(t, "{")
	assertParseINEGIError(t, `{"value":[]}`)
	assertParseINEGIError(t, `{"value":["1"]}`)
	assertParseUMAError(t, "not-a-number")
	assertNewUMADataError(t, 29999)
	assertNewUMADataError(t, 60001)
}

func TestINEGIObservations(t *testing.T) {
	client := NewINEGIClient("token", discardLogger())

	_, _, err := client.findUMAObservation(inegiResponse("bad", "2026"), 2026)
	assertError(t, err)

	_, err = client.umaDataFromResponse(inegiResponse("1", fmt.Sprint(time.Now().Year())))
	assertError(t, err)

	_, err = client.umaDataFromResponse(inegiResponse("bad", fmt.Sprint(time.Now().Year())))
	assertError(t, err)

	annual, found, err := client.parseUMAAt(inegiResponse("41000", "2026"), 1, 2026)
	assertNoError(t, err)
	assertBool(t, found, false)
	assertFloat(t, annual, 0)

	annual, found, err = client.parseUMAAt(inegiResponse("NA", "2026"), 0, 2026)
	assertNoError(t, err)
	assertBool(t, found, false)
	assertFloat(t, annual, 0)

	annual, year, err := client.findUMAObservation(inegiResponse("41000", "2025"), 2026)
	assertNoError(t, err)
	assertFloat(t, annual, 41000)
	assertInt(t, year, 2025)

	_, _, err = client.latestUMAObservation(inegiResponse("NA", "2025"))
	assertError(t, err)

	_, _, err = client.latestUMAObservation(inegiResponse("bad", "2025"))
	assertError(t, err)

	_, _, err = client.latestUMAObservation(inegiResponse("41000", "bad"))
	assertError(t, err)
}

func TestStripJSONPCallback(t *testing.T) {
	assertString(t, stripJSONPCallback(`callback({"ok":true});`), `{"ok":true}`)
	assertString(t, stripJSONPCallback(`{"ok":true}`), `{"ok":true}`)
}

func inegiResponse(value string, year string) INEGIResponse {
	var data INEGIResponse
	data.Value = []string{value}
	data.Dimension.Periods.Category.Label = []struct {
		Key   string `json:"Key"`
		Value string `json:"Value"`
	}{{Key: "P1", Value: year}}
	return data
}

func assertParseINEGIError(t *testing.T, body string) {
	t.Helper()

	_, err := parseINEGIResponse([]byte(body))
	assertError(t, err)
}

func assertParseUMAError(t *testing.T, value string) {
	t.Helper()

	_, err := parseUMAAnnual(value)
	assertError(t, err)
}

func assertNewUMADataError(t *testing.T, annual float64) {
	t.Helper()

	_, err := newUMAData(2026, annual)
	assertError(t, err)
}

type errBody struct{}

func (errBody) Read(_ []byte) (int, error) {
	return 0, context.Canceled
}

func (errBody) Close() error {
	return nil
}

func assertInt(t *testing.T, got int, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("got %d; want %d", got, want)
	}
}

func assertBool(t *testing.T, got bool, want bool) {
	t.Helper()
	if got != want {
		t.Fatalf("got %t; want %t", got, want)
	}
}

func withINEGIAPIEndpoint(t *testing.T, endpoint string) {
	t.Helper()

	previous := inegiAPIEndpoint
	inegiAPIEndpoint = endpoint
	t.Cleanup(func() {
		inegiAPIEndpoint = previous
	})
}
