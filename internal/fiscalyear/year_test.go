package fiscalyear

import (
	"testing"
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/database"
)

func TestLabel(t *testing.T) {
	now := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		fiscalYear database.FiscalYear
		want       int
	}{
		{name: "uses fiscal year", fiscalYear: database.FiscalYear{Year: 2027}, want: 2027},
		{name: "falls back to current year", fiscalYear: database.FiscalYear{}, want: 2026},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Label(tt.fiscalYear, now); got != tt.want {
				t.Fatalf("got %d; want %d", got, tt.want)
			}
		})
	}
}

func TestCurrentLabel(t *testing.T) {
	if got := CurrentLabel(database.FiscalYear{Year: 2027}); got != 2027 {
		t.Fatalf("got %d; want 2027", got)
	}
}
