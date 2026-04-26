package funcs

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var printer = message.NewPrinter(language.English)

var TemplateFuncs = template.FuncMap{

	"now":            time.Now,
	"timeSince":      time.Since,
	"timeUntil":      time.Until,
	"formatTime":     formatTime,
	"approxDuration": approxDuration,

	"uppercase": strings.ToUpper,
	"lowercase": strings.ToLower,
	"pluralize": pluralize,
	"slugify":   slugify,
	"safeHTML":  safeHTML,

	"join": strings.Join,

	"add":         add,
	"incr":        incr,
	"decr":        decr,
	"formatInt":   formatInt,
	"formatFloat": formatFloat,

	"yesNo": yesNo,

	"urlSetParam": urlSetParam,
	"urlDelParam": urlDelParam,

	"dict":     dict,
	"viteTags": viteTags,
}

func formatTime(format string, t time.Time) string {
	return t.Format(format)
}

func approxDuration(d time.Duration) string {
	for _, unit := range durationUnits {
		if d >= unit.Duration {
			return formatDurationUnit(d, unit)
		}
	}
	return "less than 1 second"
}

type durationUnit struct {
	Duration time.Duration
	Singular string
	Plural   string
}

var durationUnits = []durationUnit{
	{Duration: 365 * 24 * time.Hour, Singular: "year", Plural: "years"},
	{Duration: 24 * time.Hour, Singular: "day", Plural: "days"},
	{Duration: time.Hour, Singular: "hour", Plural: "hours"},
	{Duration: time.Minute, Singular: "minute", Plural: "minutes"},
	{Duration: time.Second, Singular: "second", Plural: "seconds"},
}

func formatDurationUnit(d time.Duration, unit durationUnit) string {
	count := int(math.Round(float64(d) / float64(unit.Duration)))
	if count == 1 {
		return fmt.Sprintf("1 %s", unit.Singular)
	}
	return fmt.Sprintf("%d %s", count, unit.Plural)
}

func pluralize(count any, singular string, plural string) (string, error) {
	n, err := toInt64(count)
	if err != nil {
		return "", err
	}

	if n == 1 {
		return singular, nil
	}

	return plural, nil
}

func slugify(s string) string {
	var buf bytes.Buffer

	for _, r := range s {
		if slug, ok := slugRune(r); ok {
			buf.WriteRune(slug)
		}
	}

	return buf.String()
}

func slugRune(r rune) (rune, bool) {
	if r > unicode.MaxASCII {
		return 0, false
	}
	return asciiSlugRune(r)
}

func asciiSlugRune(r rune) (rune, bool) {
	if unicode.IsLetter(r) {
		return unicode.ToLower(r), true
	}
	if isSlugLiteral(r) {
		return r, true
	}
	if unicode.IsSpace(r) {
		return '-', true
	}
	return 0, false
}

var slugLiteralRunes = map[rune]bool{'_': true, '-': true}

func isSlugLiteral(r rune) bool {
	if unicode.IsDigit(r) {
		return true
	}
	return slugLiteralRunes[r]
}

func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

func add(a, b any) (int64, error) {
	an, err := toInt64(a)
	if err != nil {
		return 0, err
	}
	bn, err := toInt64(b)
	if err != nil {
		return 0, err
	}
	return an + bn, nil
}

func incr(i any) (int64, error) {
	n, err := toInt64(i)
	if err != nil {
		return 0, err
	}

	n++
	return n, nil
}

func decr(i any) (int64, error) {
	n, err := toInt64(i)
	if err != nil {
		return 0, err
	}

	n--
	return n, nil
}

func formatInt(i any) (string, error) {
	n, err := toInt64(i)
	if err != nil {
		return "", err
	}

	return printer.Sprintf("%d", n), nil
}

func formatFloat(f float64, dp int) string {
	format := "%." + strconv.Itoa(dp) + "f"
	return printer.Sprintf(format, f)
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}

	return "No"
}

func urlSetParam(u *url.URL, key string, value any) *url.URL {
	nu := *u
	values := nu.Query()

	values.Set(key, fmt.Sprintf("%v", value))

	nu.RawQuery = values.Encode()
	return &nu
}

func urlDelParam(u *url.URL, key string) *url.URL {
	nu := *u
	values := nu.Query()

	values.Del(key)

	nu.RawQuery = values.Encode()
	return &nu
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict requires an even number of arguments (key-value pairs), got %d", len(values))
	}

	result := make(map[string]any, len(values)/2)

	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key must be a string, got %T", values[i])
		}
		result[key] = values[i+1]
	}

	return result, nil
}

func toInt64(i any) (int64, error) {
	return int64FromValue(reflect.ValueOf(i), i)
}

func int64FromValue(value reflect.Value, original any) (int64, error) {
	if !value.IsValid() {
		return 0, fmt.Errorf("unable to convert type %T to int", original)
	}
	if value.Kind() == reflect.String {
		return strconv.ParseInt(value.String(), 10, 64)
	}
	return numericInt64(value, original)
}

func numericInt64(value reflect.Value, original any) (int64, error) {
	if value.CanInt() {
		return value.Int(), nil
	}
	if value.CanUint() {
		return int64(value.Uint()), nil
	}
	return 0, fmt.Errorf("unable to convert type %T to int", original)
}
