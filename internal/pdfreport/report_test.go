package pdfreport

import (
	"errors"
	"html/template"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/database"
)

func TestRenderComparisonHTML(t *testing.T) {
	html, err := RenderComparisonHTML([]PackageResult{testPackageResult()}, database.FiscalYear{Year: 2026})

	assertNoError(t, err)
	assertContains(t, html, "Base")
}

func TestRenderComparisonHTMLUsesReportData(t *testing.T) {
	tmpl := template.Must(template.New("report.tmpl").Parse(`{{.Date}} {{.FiscalYear}} {{range .Packages}}{{.Name}}{{end}}`))
	html, err := renderComparisonHTML([]PackageResult{testPackageResult()}, database.FiscalYear{Year: 2026}, func() (*template.Template, error) {
		return tmpl, nil
	}, fixedNow)

	assertNoError(t, err)
	assertString(t, html, "02 Jan 2025 2026 Base")
}

func TestRenderComparisonHTMLReportsParseErrors(t *testing.T) {
	_, err := renderComparisonHTML(nil, database.FiscalYear{}, func() (*template.Template, error) {
		return nil, errors.New("parse failed")
	}, fixedNow)

	assertErrorContains(t, err, "parse failed")
}

func TestRenderComparisonHTMLReportsExecutionErrors(t *testing.T) {
	tmpl := template.Must(template.New("report.tmpl").Funcs(template.FuncMap{
		"fail": func() (string, error) {
			return "", errors.New("execute failed")
		},
	}).Parse(`{{fail}}`))

	_, err := renderComparisonHTML(nil, database.FiscalYear{}, func() (*template.Template, error) {
		return tmpl, nil
	}, fixedNow)

	assertErrorContains(t, err, "failed to execute PDF template")
	assertErrorContains(t, err, "execute failed")
}

func TestParseReportTemplateFSReportsMissingTemplate(t *testing.T) {
	_, err := parseReportTemplateFS(fstest.MapFS{}, "missing.tmpl")

	assertErrorContains(t, err, "failed to parse PDF template at missing.tmpl")
}

func TestReportHelpers(t *testing.T) {
	calc := &database.SalaryCalculation{
		AguinaldoNet:             1,
		PrimaVacacionalNet:       2,
		FondoAhorroYearly:        3,
		InfonavitEmployerMonthly: 4,
		IMSSEmployerMonthly:      5,
		OtherBenefits:            []database.OtherBenefitResult{{Cadence: "annual"}},
	}

	assertString(t, formatFloat(12345.678, 2), "12,345.68")
	assertFloat(t, safeDiv(12, 3), 4)
	assertFloat(t, safeDiv(12, 0), 0)
	assertFloat(t, mul(3, 4), 12)
	assertBool(t, hasAnnualBenefits(calc), true)
	assertBool(t, hasAnnualBenefits(nil), false)
	assertBool(t, hasEmployerContributions(calc), true)
	assertBool(t, hasEmployerContributions(nil), false)
}

func TestAnnualBenefitHelpers(t *testing.T) {
	assertBool(t, hasFixedAnnualBenefits(&database.SalaryCalculation{AguinaldoNet: 1}), true)
	assertBool(t, hasFixedAnnualBenefits(&database.SalaryCalculation{PrimaVacacionalNet: 1}), true)
	assertBool(t, hasFixedAnnualBenefits(&database.SalaryCalculation{FondoAhorroYearly: 1}), true)
	assertBool(t, hasFixedAnnualBenefits(&database.SalaryCalculation{}), false)
	assertBool(t, hasFixedAnnualBenefits(nil), false)

	assertBool(t, hasAnnualOtherBenefits(&database.SalaryCalculation{OtherBenefits: []database.OtherBenefitResult{{Cadence: "annual"}}}), true)
	assertBool(t, hasAnnualOtherBenefits(&database.SalaryCalculation{OtherBenefits: []database.OtherBenefitResult{{Cadence: "monthly"}}}), false)
	assertBool(t, hasAnnualOtherBenefits(nil), false)
}

func TestEmployerContributionHelpers(t *testing.T) {
	assertBool(t, hasEmployerContributions(&database.SalaryCalculation{InfonavitEmployerMonthly: 1}), true)
	assertBool(t, hasEmployerContributions(&database.SalaryCalculation{IMSSEmployerMonthly: 1}), true)
	assertBool(t, hasEmployerContributions(&database.SalaryCalculation{}), false)
	assertBool(t, hasEmployerContributions(nil), false)
}

func TestNumberFormattingHelpers(t *testing.T) {
	assertString(t, addThousandsSeparator("1234567.89"), "1,234,567.89")
	assertString(t, addThousandsSeparator("1234567"), "1,234,567")
	assertString(t, addThousandsSeparator("-1234567"), "-1,234,567")

	intPart, decPart := splitNumberParts("123.45")
	assertString(t, intPart, "123")
	assertString(t, decPart, ".45")

	intPart, decPart = splitNumberParts("123")
	assertString(t, intPart, "123")
	assertString(t, decPart, "")

	unsigned, negative := unsignedNumber("-123")
	assertString(t, unsigned, "123")
	assertBool(t, negative, true)

	unsigned, negative = unsignedNumber("123")
	assertString(t, unsigned, "123")
	assertBool(t, negative, false)

	assertString(t, groupThousands("7654321"), "765,432,1")
	assertString(t, signedNumber("123", true), "-123")
	assertString(t, signedNumber("123", false), "123")
	assertString(t, reverse("abc"), "cba")
}

func testPackageResult() PackageResult {
	return PackageResult{
		Name: "Base",
		Input: PackageInput{
			Name:                   "Base",
			Regime:                 "salary",
			Currency:               "MXN",
			GrossMonthlySalary:     "100000",
			PaymentFrequency:       "monthly",
			HasAguinaldo:           true,
			HasPrimaVacacional:     true,
			HasValesDespensa:       true,
			ValesDespensaAmount:    "1000",
			HasFondoAhorro:         true,
			FondoAhorroPercent:     "10",
			UnpaidVacationDays:     "0",
			HasEquity:              true,
			InitialEquityUSD:       "10000",
			HasRefreshers:          true,
			RefresherMinUSD:        "5000",
			RefresherMaxUSD:        "7000",
			OtherBenefits:          []OtherBenefit{{Name: "Bonus", Amount: 1000, Cadence: "annual"}},
			ExchangeRate:           "20",
			HoursPerWeek:           "40",
			AguinaldoDays:          "15",
			VacationDays:           "12",
			PrimaVacacionalPercent: "25",
		},
		Calculation: &database.SalaryCalculation{YearlyNet: 1000000},
	}
}

func fixedNow() time.Time {
	return time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q; got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("got error %q; want to contain %q", err, want)
	}
}

func assertContains(t *testing.T, got string, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("got %q; want to contain %q", got, want)
	}
}

func assertString(t *testing.T, got string, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}

func assertFloat(t *testing.T, got float64, want float64) {
	t.Helper()
	if got != want {
		t.Fatalf("got %f; want %f", got, want)
	}
}

func assertBool(t *testing.T, got bool, want bool) {
	t.Helper()
	if got != want {
		t.Fatalf("got %t; want %t", got, want)
	}
}
