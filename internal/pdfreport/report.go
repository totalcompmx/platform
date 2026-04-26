package pdfreport

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"strings"
	"time"

	"github.com/jcroyoaun/totalcompmx/assets"
	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/funcs"
)

// OtherBenefit represents custom benefits.
type OtherBenefit struct {
	Name     string
	Amount   float64
	TaxFree  bool
	Currency string
	Cadence  string
}

// PackageInput represents the input details for a package.
type PackageInput struct {
	Name                   string
	Regime                 string
	Currency               string
	ExchangeRate           string
	PaymentFrequency       string
	HoursPerWeek           string
	GrossMonthlySalary     string
	HasAguinaldo           bool
	AguinaldoDays          string
	HasValesDespensa       bool
	ValesDespensaAmount    string
	HasPrimaVacacional     bool
	VacationDays           string
	PrimaVacacionalPercent string
	HasFondoAhorro         bool
	FondoAhorroPercent     string
	UnpaidVacationDays     string
	OtherBenefits          []OtherBenefit
	HasEquity              bool
	InitialEquityUSD       string
	HasRefreshers          bool
	RefresherMinUSD        string
	RefresherMaxUSD        string
}

// PackageResult represents a single package's calculation results.
type PackageResult struct {
	Name        string
	Input       PackageInput
	Calculation *database.SalaryCalculation
}

// ReportData represents the data passed to the PDF template.
type ReportData struct {
	Date     string
	Packages []PackageResult
}

func RenderComparisonHTML(packages []PackageResult) (string, error) {
	return renderComparisonHTML(packages, parseReportTemplate, time.Now)
}

func renderComparisonHTML(packages []PackageResult, parse func() (*template.Template, error), now func() time.Time) (string, error) {
	tmpl, err := parse()
	if err != nil {
		return "", err
	}

	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, reportData(packages, now())); err != nil {
		return "", fmt.Errorf("failed to execute PDF template: %w", err)
	}

	return htmlBuf.String(), nil
}

func reportData(packages []PackageResult, now time.Time) ReportData {
	return ReportData{
		Date:     now.Format("02 Jan 2006"),
		Packages: packages,
	}
}

func parseReportTemplate() (*template.Template, error) {
	return parseReportTemplateFS(assets.EmbeddedFiles, reportTemplatePath())
}

func parseReportTemplateFS(fsys fs.FS, templatePath string) (*template.Template, error) {
	tmpl, err := template.New("report.tmpl").Funcs(reportTemplateFuncs()).ParseFS(fsys, templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDF template at %s: %w", templatePath, err)
	}

	return tmpl, nil
}

func reportTemplatePath() string {
	return "templates/pdf/report.tmpl"
}

func reportTemplateFuncs() template.FuncMap {
	funcMap := template.FuncMap{}
	for name, fn := range funcs.TemplateFuncs {
		funcMap[name] = fn
	}

	funcMap["formatFloat"] = formatFloat
	funcMap["div"] = safeDiv
	funcMap["mul"] = mul
	funcMap["hasAnnualBenefits"] = hasAnnualBenefits
	funcMap["hasEmployerContributions"] = hasEmployerContributions

	return funcMap
}

func formatFloat(f float64, decimals int) string {
	formatted := fmt.Sprintf(fmt.Sprintf("%%.%df", decimals), f)
	return addThousandsSeparator(formatted)
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}

	return a / b
}

func mul(a, b float64) float64 {
	return a * b
}

func hasAnnualBenefits(calc *database.SalaryCalculation) bool {
	return hasFixedAnnualBenefits(calc) || hasAnnualOtherBenefits(calc)
}

func hasFixedAnnualBenefits(calc *database.SalaryCalculation) bool {
	if calc == nil {
		return false
	}

	return calc.AguinaldoNet > 0 || calc.PrimaVacacionalNet > 0 || calc.FondoAhorroYearly > 0
}

func hasAnnualOtherBenefits(calc *database.SalaryCalculation) bool {
	if calc == nil {
		return false
	}

	for _, benefit := range calc.OtherBenefits {
		if benefit.Cadence == "annual" {
			return true
		}
	}

	return false
}

func hasEmployerContributions(calc *database.SalaryCalculation) bool {
	if calc == nil {
		return false
	}

	return calc.InfonavitEmployerMonthly > 0 || calc.IMSSEmployerMonthly > 0
}

func addThousandsSeparator(s string) string {
	intPart, decPart := splitNumberParts(s)
	intPart, negative := unsignedNumber(intPart)
	finalInt := reverse(groupThousands(reverse(intPart)))
	return signedNumber(finalInt, negative) + decPart
}

func splitNumberParts(s string) (string, string) {
	intPart, decPart, found := strings.Cut(s, ".")
	if !found {
		return intPart, ""
	}

	return intPart, "." + decPart
}

func unsignedNumber(intPart string) (string, bool) {
	if strings.HasPrefix(intPart, "-") {
		return intPart[1:], true
	}

	return intPart, false
}

func groupThousands(reversed string) string {
	var result []rune
	for i, char := range reversed {
		if i > 0 && i%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, char)
	}

	return string(result)
}

func signedNumber(intPart string, negative bool) string {
	if negative {
		return "-" + intPart
	}

	return intPart
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
