package fiscalyear

import (
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/database"
)

func Label(fiscalYear database.FiscalYear, now time.Time) int {
	if fiscalYear.Year > 0 {
		return fiscalYear.Year
	}

	return now.Year()
}

func CurrentLabel(fiscalYear database.FiscalYear) int {
	return Label(fiscalYear, time.Now())
}
