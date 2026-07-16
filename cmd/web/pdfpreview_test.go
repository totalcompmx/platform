package main

// Preview harness for PDF template design work: renders representative
// scenarios through the real pipeline (calculation engine + report template).
// Skipped unless PDF_PREVIEW_DIR is set:
//
//	PDF_PREVIEW_DIR=/tmp go test ./cmd/web/ -run TestGeneratePDFPreviews -count=1

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePDFPreviews(t *testing.T) {
	dir := os.Getenv("PDF_PREVIEW_DIR")
	if dir == "" {
		t.Skip("PDF_PREVIEW_DIR not set")
	}

	app := newTestApplication(t)
	fiscalYear := testFiscalYear()

	scenarios := map[string]url.Values{
		"single.pdf": {
			"PackageName[]":        {"Mi Oferta"},
			"Regime[]":             {"sueldos_salarios"},
			"GrossMonthlySalary[]": {"45000"},
			"HasAguinaldo[]":       {"0"},
			"AguinaldoDays[]":      {"15"},
		},
		"comparison.pdf": {
			"PackageName[]":            {"Empresa Grande SA", "Startup RESICO"},
			"Regime[]":                 {"sueldos_salarios", "resico"},
			"GrossMonthlySalary[]":     {"85000", "95000"},
			"HasAguinaldo[]":           {"0"},
			"AguinaldoDays[]":          {"30", ""},
			"HasValesDespensa[]":       {"0"},
			"ValesDespensaAmount[]":    {"3439", ""},
			"HasPrimaVacacional[]":     {"0"},
			"VacationDays[]":           {"20", ""},
			"PrimaVacacionalPercent[]": {"50", ""},
			"HasFondoAhorro[]":         {"0"},
			"FondoAhorroPercent[]":     {"13", ""},
			"UnpaidVacationDays[]":     {"", "10"},
			"OtherBenefitName-0[]":     {"Seguro Gastos Médicos"},
			"OtherBenefitAmount-0[]":   {"2500"},
			"OtherBenefitCadence-0[]":  {"monthly"},
			"OtherBenefitCurrency-0[]": {"MXN"},
			"OtherBenefitTaxFree-0[]":  {"true"},
		},
		"triple.pdf": {
			"PackageName[]":         {"Oferta Alfa", "Oferta Beta", "Oferta Gamma"},
			"Regime[]":              {"sueldos_salarios", "sueldos_salarios", "resico"},
			"GrossMonthlySalary[]":  {"60000", "75000", "70000"},
			"HasAguinaldo[]":        {"0", "1"},
			"AguinaldoDays[]":       {"15", "30", ""},
			"HasValesDespensa[]":    {"1"},
			"ValesDespensaAmount[]": {"", "3000", ""},
		},
		"usd_equity.pdf": {
			"PackageName[]":        {"FAANG Remote", "Nacional"},
			"Regime[]":             {"sueldos_salarios", "sueldos_salarios"},
			"GrossMonthlySalary[]": {"5000", "90000"},
			"Currency[]":           {"USD", "MXN"},
			"ExchangeRate[]":       {"17.5", ""},
			"HasEquity[]":          {"0"},
			"InitialEquityUSD[]":   {"60000", ""},
			"HasRefreshers[]":      {"0"},
			"RefresherMinUSD[]":    {"10000", ""},
			"RefresherMaxUSD[]":    {"20000", ""},
			"HasAguinaldo[]":       {"0", "1"},
			"AguinaldoDays[]":      {"15", "30"},
		},
	}

	for name, form := range scenarios {
		payload := newHomePostPayload(form)
		results, err := app.buildHomeResults(payload, fiscalYear)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		pdfBytes, err := generateComparisonReport(buildPDFPackages(results.Results, results.PackageInputs), fiscalYear)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if err := os.WriteFile(filepath.Join(dir, name), pdfBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d bytes, %d packages)", name, len(pdfBytes), len(results.Results))
	}
}
