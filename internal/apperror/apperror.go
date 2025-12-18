package apperror

import "errors"

// Kind describes a stable error category that can be mapped to HTTP status codes.
type Kind string

const (
	KindNotFound   Kind = "not_found"
	KindValidation Kind = "validation"
	KindConflict   Kind = "conflict"
)

// Error is a typed error with a stable Kind and a human-readable message.
// Msg should be safe to return to clients for Validation/NotFound/Conflict.
type Error struct {
	Kind Kind
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Msg != "" {
		return e.Msg
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Kind)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(kind Kind, msg string, err error) error {
	return &Error{Kind: kind, Msg: msg, Err: err}
}

func NotFound(msg string, err error) error   { return New(KindNotFound, msg, err) }
func Validation(msg string, err error) error { return New(KindValidation, msg, err) }
func Conflict(msg string, err error) error   { return New(KindConflict, msg, err) }

func Is(err error, kind Kind) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Kind == kind
}
