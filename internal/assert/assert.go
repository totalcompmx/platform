package assert

import (
	"errors"
	"reflect"
	"regexp"
	"testing"
)

type testingT interface {
	Helper()
	Errorf(string, ...any)
	Fatalf(string, ...any)
}

func Equal[T any](t *testing.T, got, want T) {
	equal(t, got, want)
}

func equal[T any](t testingT, got, want T) {
	t.Helper()
	if !isEqual(got, want) {
		t.Errorf("got: %v; want: %v", got, want)
	}
}

func NotEqual[T any](t *testing.T, got, want T) {
	notEqual(t, got, want)
}

func notEqual[T any](t testingT, got, want T) {
	t.Helper()
	if isEqual(got, want) {
		t.Errorf("got: %v; expected values to be different", got)
	}
}

func True(t *testing.T, got bool) {
	trueValue(t, got)
}

func trueValue(t testingT, got bool) {
	t.Helper()
	if !got {
		t.Errorf("got: false; want: true")
	}
}

func False(t *testing.T, got bool) {
	falseValue(t, got)
}

func falseValue(t testingT, got bool) {
	t.Helper()
	if got {
		t.Errorf("got: true; want: false")
	}
}

func Nil(t *testing.T, got any) {
	nilValue(t, got)
}

func nilValue(t testingT, got any) {
	t.Helper()
	if !isNil(got) {
		t.Errorf("got: %v; want: nil", got)
	}
}

func NotNil(t *testing.T, got any) {
	notNilValue(t, got)
}

func notNilValue(t testingT, got any) {
	t.Helper()
	if isNil(got) {
		t.Errorf("got: nil; want: non-nil")
	}
}

func ErrorIs(t *testing.T, got, want error) {
	errorIs(t, got, want)
}

func errorIs(t testingT, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("got: %v; want: %v", got, want)
	}
}

func ErrorAs(t *testing.T, got error, target any) {
	errorAs(t, got, target)
}

func errorAs(t testingT, got error, target any) {
	t.Helper()
	if got == nil {
		t.Errorf("got: nil; want assignable to: %T", target)
		return
	}
	if !errors.As(got, target) {
		t.Errorf("got: %v; want assignable to: %T", got, target)
	}
}

func MatchesRegexp(t *testing.T, got, pattern string) {
	matchesRegexp(t, got, pattern)
}

func matchesRegexp(t testingT, got, pattern string) {
	t.Helper()
	matched, err := regexp.MatchString(pattern, got)
	if err != nil {
		t.Fatalf("unable to parse regexp pattern %s: %s", pattern, err.Error())
		return
	}
	if !matched {
		t.Errorf("got: %q; want to match %q", got, pattern)
	}
}

func isEqual[T any](got, want T) bool {
	if isNil(got) && isNil(want) {
		return true
	}
	if equalable, ok := any(got).(interface{ Equal(T) bool }); ok {
		return equalable.Equal(want)
	}
	return reflect.DeepEqual(got, want)
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	}
	return false
}
