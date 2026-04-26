package assert

import (
	"errors"
	"fmt"
	"testing"
)

type fakeTestingT struct {
	errors []string
	fatals []string
}

func (t *fakeTestingT) Helper() {}

func (t *fakeTestingT) Errorf(format string, args ...any) {
	t.errors = append(t.errors, fmt.Sprintf(format, args...))
}

func (t *fakeTestingT) Fatalf(format string, args ...any) {
	t.fatals = append(t.fatals, fmt.Sprintf(format, args...))
}

type equalString string

func (s equalString) Equal(other equalString) bool {
	return s == other
}

type customError struct{}

func (customError) Error() string {
	return "custom"
}

func TestAssertionsPass(t *testing.T) {
	err := customError{}
	wrapped := fmt.Errorf("wrapped: %w", err)

	Equal(t, 1, 1)
	NotEqual(t, 1, 2)
	True(t, true)
	False(t, false)
	Nil(t, []string(nil))
	NotNil(t, []string{"value"})
	ErrorIs(t, wrapped, err)
	ErrorAs(t, wrapped, new(customError))
	MatchesRegexp(t, "abc123", `^[a-z]+\d+$`)
}

func TestAssertionsRecordFailures(t *testing.T) {
	err := errors.New("target")
	wrapped := errors.New("wrapped")

	tests := []func(*fakeTestingT){
		func(ft *fakeTestingT) { equal(ft, 1, 2) },
		func(ft *fakeTestingT) { notEqual(ft, 1, 1) },
		func(ft *fakeTestingT) { trueValue(ft, false) },
		func(ft *fakeTestingT) { falseValue(ft, true) },
		func(ft *fakeTestingT) { nilValue(ft, "value") },
		func(ft *fakeTestingT) { notNilValue(ft, nil) },
		func(ft *fakeTestingT) { errorIs(ft, wrapped, err) },
		func(ft *fakeTestingT) { errorAs(ft, nil, new(customError)) },
		func(ft *fakeTestingT) { errorAs(ft, wrapped, new(customError)) },
		func(ft *fakeTestingT) { matchesRegexp(ft, "abc", `^\d+$`) },
	}

	for _, run := range tests {
		ft := &fakeTestingT{}
		run(ft)
		if len(ft.errors) != 1 {
			t.Fatalf("got %d errors; want 1", len(ft.errors))
		}
	}
}

func TestMatchesRegexpFatal(t *testing.T) {
	ft := &fakeTestingT{}
	matchesRegexp(ft, "abc", "[")

	if len(ft.fatals) != 1 {
		t.Fatalf("got %d fatals; want 1", len(ft.fatals))
	}
}

func TestIsEqual(t *testing.T) {
	Equal(t, true, isEqual([]int(nil), []int(nil)))
	Equal(t, true, isEqual(equalString("a"), equalString("a")))
	Equal(t, false, isEqual(equalString("a"), equalString("b")))
}

func TestIsNil(t *testing.T) {
	var ch chan int
	var fn func()
	var iface any
	var mp map[string]int
	var ptr *int
	var slice []int

	Equal(t, true, isNil(ch))
	Equal(t, true, isNil(fn))
	Equal(t, true, isNil(iface))
	Equal(t, true, isNil(mp))
	Equal(t, true, isNil(ptr))
	Equal(t, true, isNil(slice))
	Equal(t, false, isNil(0))
}
