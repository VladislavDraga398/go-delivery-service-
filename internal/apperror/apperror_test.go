package apperror

import (
	"errors"
	"fmt"
	"testing"
)

func TestError_ErrorMessagePriority(t *testing.T) {
	base := errors.New("base")
	err := &Error{Kind: KindValidation, Msg: "msg", Err: base}
	if err.Error() != "msg" {
		t.Fatalf("expected msg, got %q", err.Error())
	}
}

func TestError_ErrorFallsBackToWrapped(t *testing.T) {
	base := errors.New("base")
	err := &Error{Kind: KindValidation, Err: base}
	if err.Error() != "base" {
		t.Fatalf("expected base, got %q", err.Error())
	}
}

func TestError_ErrorFallsBackToKind(t *testing.T) {
	err := &Error{Kind: KindNotFound}
	if err.Error() != string(KindNotFound) {
		t.Fatalf("expected kind string, got %q", err.Error())
	}
}

func TestError_Unwrap(t *testing.T) {
	base := errors.New("base")
	err := &Error{Kind: KindValidation, Err: base}
	if !errors.Is(err, base) {
		t.Fatalf("expected wrapped error to be reachable via errors.Is")
	}
}

func TestIs_MatchesWrappedKind(t *testing.T) {
	err := NotFound("x", nil)
	wrapped := fmt.Errorf("wrap: %w", err)
	if !Is(wrapped, KindNotFound) {
		t.Fatalf("expected Is to match wrapped kind")
	}
	if Is(wrapped, KindValidation) {
		t.Fatalf("expected Is to be false for different kind")
	}
}
