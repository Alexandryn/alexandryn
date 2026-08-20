//go:build negativecompile

package domain_test

// This file is never built by `go test ./...` — the negativecompile tag
// is never passed. It exists to prove FR-3's barrier is a compile-time
// property, not a runtime one: a passing runtime test cannot demonstrate
// the *absence* of a code path, only a build failure can.
//
// Verify with go vet (go build ignores _test.go files entirely, so it
// can't be used to prove this) — it MUST fail:
//
//	go vet -tags negativecompile ./internal/domain/...
//
// Confirmed failure: "cannot use progress (variable of struct type
// domain.ReadingProgressUpdated) as domain.PublicEvent value in argument
// to logSink: domain.ReadingProgressUpdated does not implement
// domain.PublicEvent (missing method isPublicEvent)".

import (
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func logSink(e domain.PublicEvent) {
	_ = e
}

func thisMustNotCompile() {
	progress := domain.NewReadingProgressUpdated("progress-1", time.Now())
	logSink(progress) // SensitiveEvent handed to a PublicEvent-typed sink
}
