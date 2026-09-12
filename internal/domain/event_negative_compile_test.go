//go:build negativecompile

package domain_test

// This file is not built during standard `go test ./...` runs — the negativecompile tag
// is not passed by default. It proves that sensitive events cannot be passed to a
// public event sink at compile time:
//
//	go vet -tags negativecompile ./internal/domain/...
//
// Expected compile failure: "cannot use progress (variable of struct type
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
