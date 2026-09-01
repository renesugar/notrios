package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/renesugar/notrios/internal/store"
)

// Kind classifies a failure in transport-neutral terms.
//
// It exists so that HTTP, MCP, and the C ABI can each map one classification
// into their own vocabulary instead of re-deriving it from error strings. The
// REST adapter's existing status mapping is the reference reading: KindNotFound
// is 404, KindConflict is 409, KindPreconditionRequired is 428, and so on. That
// mapping stays in the adapter, because a status code is a transport detail.
type Kind int

const (
	// KindInternal is an unclassified failure. It is the zero value so that a
	// carelessly constructed Error is never mistaken for a client mistake.
	KindInternal Kind = iota
	KindNotFound
	KindConflict
	KindPreconditionRequired
	KindInvalidInput
	KindInvalidCursor
	KindNameConflict
	KindForbidden
	KindUnavailable
	KindCanceled
)

var kindNames = map[Kind]string{
	KindInternal:             "internal",
	KindNotFound:             "not_found",
	KindConflict:             "conflict",
	KindPreconditionRequired: "precondition_required",
	KindInvalidInput:         "invalid_input",
	KindInvalidCursor:        "invalid_cursor",
	KindNameConflict:         "name_conflict",
	KindForbidden:            "forbidden",
	KindUnavailable:          "unavailable",
	KindCanceled:             "canceled",
}

// String returns the stable machine-readable name of the kind. The names are
// part of the facade contract: the C ABI reports them across the boundary, so
// renaming one is a breaking change.
func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}
	return "internal"
}

// Error is the typed failure returned by every facade operation.
type Error struct {
	Kind    Kind
	Op      string
	Message string

	wrapped error
}

func (e *Error) Error() string {
	switch {
	case e.Op != "" && e.Message != "":
		return fmt.Sprintf("%s: %s: %s", e.Op, e.Kind, e.Message)
	case e.Op != "":
		return fmt.Sprintf("%s: %s", e.Op, e.Kind)
	case e.Message != "":
		return fmt.Sprintf("%s: %s", e.Kind, e.Message)
	default:
		return e.Kind.String()
	}
}

func (e *Error) Unwrap() error { return e.wrapped }

// Is lets errors.Is compare against the package sentinels by kind, so callers
// can write errors.Is(err, application.ErrNotFound) without reaching for a
// type assertion or matching on a message.
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && other.Kind == e.Kind && other.Op == "" && other.Message == ""
}

func sentinel(kind Kind) *Error { return &Error{Kind: kind} }

// Sentinels for errors.Is. They carry no operation or message so that any
// error of the same kind matches.
var (
	ErrNotFound             = sentinel(KindNotFound)
	ErrConflict             = sentinel(KindConflict)
	ErrPreconditionRequired = sentinel(KindPreconditionRequired)
	ErrInvalidInput         = sentinel(KindInvalidInput)
	ErrInvalidCursor        = sentinel(KindInvalidCursor)
	ErrNameConflict         = sentinel(KindNameConflict)
	ErrForbidden            = sentinel(KindForbidden)
	ErrUnavailable          = sentinel(KindUnavailable)
	ErrInternal             = sentinel(KindInternal)
)

// KindOf reports the classification of an error, so an adapter can switch on
// one value instead of testing each sentinel in turn.
func KindOf(err error) Kind {
	if err == nil {
		return KindInternal
	}
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Kind
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return KindCanceled
	}
	return KindInternal
}

// invalid builds a KindInvalidInput error for a caller mistake the facade
// catches before it reaches the repository.
func invalid(op, message string) *Error {
	return &Error{Kind: KindInvalidInput, Op: op, Message: message}
}

// wrap converts a repository error into a facade error.
//
// Context cancellation passes through unchanged. A caller that cancelled needs
// errors.Is(err, context.Canceled) to keep working, and re-typing it here would
// silently break every select-driven caller in the codebase.
func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var already *Error
	if errors.As(err, &already) {
		return err
	}
	return &Error{Kind: kindFromStore(err), Op: op, Message: err.Error(), wrapped: err}
}

// kindFromStore is the single place the store's error vocabulary is
// translated. Every other file in this package goes through wrap.
//
// The order mirrors writeStoreError in internal/httpapi so the REST adapter's
// behavior can be compared against this table line by line. The sentinels are
// disjoint, so the order is presentational rather than load-bearing.
func kindFromStore(err error) Kind {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return KindNotFound
	case errors.Is(err, store.ErrPreconditionRequired):
		return KindPreconditionRequired
	case errors.Is(err, store.ErrConflict):
		return KindConflict
	case errors.Is(err, store.ErrInvalidInput):
		return KindInvalidInput
	case errors.Is(err, store.ErrInvalidCursor):
		return KindInvalidCursor
	case errors.Is(err, store.ErrNameConflict):
		return KindNameConflict
	case errors.Is(err, store.ErrProtected):
		return KindForbidden
	case errors.Is(err, store.ErrResourceUnavailable):
		return KindUnavailable
	case errors.Is(err, store.ErrPhysicalRestoreInProgress):
		return KindUnavailable
	default:
		return KindInternal
	}
}
