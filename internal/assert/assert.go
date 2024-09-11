// Package assert provides test assertion helpers.
//
// Assertion functions report failures using t.Errorf to allow the containing
// test to continue.
//
// Each function returns a bool indicating whether it was satisfied to support
// conditional assertions.
package assert

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Equal asserts that two values are equal.
func Equal[T any](t *testing.T, got, want T) bool {
	t.Helper()

	diff := cmp.Diff(got, want)
	if diff == "" {
		return true
	}

	msg := "expected values to be equal"

	t.Errorf("%s (-want +got):\n%s", msg, diff)
	return false
}

// NoError asserts that err is nil.
func NoError(t *testing.T, err error) bool {
	t.Helper()

	if err == nil {
		return true
	}

	diff := fmt.Sprintf("- %v\n+ %#+v", nil, err)

	msg := "expected error to be nil"

	t.Errorf("%s (-want +got):\n%s", msg, diff)
	return false
}

// ErrorAs asserts that an error in err's chain matches target using errors.As.
func ErrorAs[E error](t *testing.T, err error) (E, bool) {
	t.Helper()

	var want E
	if errors.As(err, &want) {
		return want, true
	}

	diff := fmt.Sprintf("- %T\n+ %#+v\n+ e%q", want, err, err.Error())

	msg := fmt.Sprintf("got error of type %T, wanted %T in error chain", err, want)

	t.Errorf("%s (-want +got):\n%s", msg, diff)
	return want, false
}

// ErrorIs asserts that an error in err's chain matches target using errors.Is.
func ErrorIs(t *testing.T, got, want error) bool {
	t.Helper()

	if errors.Is(got, want) {
		return true
	}

	diff := cmp.Diff(got, want)

	msg := fmt.Sprintf("got error of type %T, wanted %T from unwrap", got, want)

	t.Errorf("%s (-want +got):\n%s", msg, diff)
	return false
}
