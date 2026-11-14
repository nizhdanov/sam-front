package apperr

import (
	"errors"
	"fmt"
)

type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindNotFound
	KindConflict
	KindUnauthenticated
	KindForbidden
	KindUnavailable
	KindRateLimited
	KindTooLarge
	KindTimeout
)

func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "invalid_argument"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindUnauthenticated:
		return "unauthenticated"
	case KindForbidden:
		return "forbidden"
	case KindUnavailable:
		return "unavailable"
	case KindRateLimited:
		return "rate_limited"
	case KindTooLarge:
		return "payload_too_large"
	case KindTimeout:
		return "timeout"
	default:
		return "internal"
	}
}

type Error struct {
	Kind Kind
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

func New(kind Kind, msg string) *Error { return &Error{Kind: kind, Msg: msg} }

func Wrap(err error, kind Kind, msg string) *Error { return &Error{Kind: kind, Msg: msg, Err: err} }

func Invalid(msg string) *Error { return New(KindInvalid, msg) }

func NotFound(msg string) *Error { return New(KindNotFound, msg) }

func Conflict(msg string) *Error { return New(KindConflict, msg) }

func Unauthenticated(msg string) *Error { return New(KindUnauthenticated, msg) }

func Forbidden(msg string) *Error { return New(KindForbidden, msg) }

func Unavailable(msg string) *Error { return New(KindUnavailable, msg) }

func RateLimited(msg string) *Error { return New(KindRateLimited, msg) }

func TooLarge(msg string) *Error { return New(KindTooLarge, msg) }

func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}

func Message(err error) string {
	var e *Error
	if errors.As(err, &e) && e.Kind != KindInternal {
		return e.Msg
	}
	return "internal error"
}
