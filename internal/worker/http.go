package worker

import (
	"fmt"
	"log/slog"
	"net/http"
)

func fetchClientResponse(httpClient *http.Client, logger *slog.Logger, req *http.Request, logMessage string, source string) ([]byte, error) {
	logger.Info(logMessage)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", source, err)
	}
	defer resp.Body.Close()

	return readOKResponse(resp, source)
}
