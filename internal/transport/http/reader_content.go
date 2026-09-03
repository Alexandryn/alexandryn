package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/reader/content"
)

// readerContentPerRequestTimeout bounds every content request regardless
// of how long FR-3's cache holds a book open (backend-reader-content.md
// FR-4).
const readerContentPerRequestTimeout = 30 * time.Second

// readerContentCSP is the response policy a sandboxed client leans on as
// a second, independent layer over its own iframe sandbox
// (backend-reader-content.md FR-9).
const readerContentCSP = "default-src 'self'; script-src 'none'; object-src 'none'"

// ReaderContentHandler serves one sanitised entry from inside an owned
// Edition's EPUB: GET /api/v1/library/editions/{editionId}/reader/content/{path...}
// (backend-reader-content.md FR-1). The content cache is constructed once
// when persistence is ready and held on poolRef; a request before it is
// ready gets a 503.
func ReaderContentHandler(poolRef *PoolRef, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())

		cache, ok := poolRef.GetReaderContentCache()
		if !ok {
			WriteError(w, domain.Unavailable, "the reader is not ready yet", correlationID)
			return
		}

		editionID := r.PathValue("editionId")
		if editionID == "" {
			WriteError(w, domain.InvalidInput, "editionId is required", correlationID)
			return
		}

		resourcePath := r.PathValue("path")
		if err := content.ValidateResourcePath(resourcePath); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}

		// The edition's bytes are served only to an authenticated member
		// of a library that owns this edition (AUDIT-0012-C1,
		// backend-reader-content.md FR-1 amendment). The auth middleware
		// has already validated X-Library-Id against the token; here we
		// confirm the edition is in that library. "Not in your library"
		// and "does not exist" return the same NotFound — no
		// cross-library existence oracle. This gate fails closed: if the
		// ownership checker is not wired, the content is not served.
		deps, ready := poolRef.GetReadingAPI()
		if !ready || deps.LibraryEntries == nil {
			WriteError(w, domain.Unavailable, "the reader is not ready yet", correlationID)
			return
		}
		if UserFromContext(r.Context()) == nil {
			WriteError(w, domain.Unauthorized, "authentication is required", correlationID)
			return
		}
		inLib, err := deps.LibraryEntries.EditionInLibrary(r.Context(), domain.EditionID(editionID), ActiveLibraryFromContext(r.Context()))
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		if !inLib {
			WriteError(w, domain.NotFound, "no such edition in your library", correlationID)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), readerContentPerRequestTimeout)
		defer cancel()

		handle, err := cache.Acquire(ctx, domain.EditionID(editionID))
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				WriteError(w, domain.Unavailable, "reading this book timed out", correlationID)
				return
			}
			writeDomainError(w, err, correlationID)
			return
		}
		defer handle.Release()

		entry, err := content.FindEntry(handle.Reader(), resourcePath)
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}

		data, err := content.ReadEntry(entry)
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}

		kind, contentType, err := content.Classify(entry.Name, data)
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}

		var body []byte
		var report content.SanitizeReport
		switch kind {
		case content.KindHTML:
			body, report = content.SanitizeHTML(data)
		case content.KindCSS:
			body, report = content.SanitizeCSS(data)
		default:
			body = data
		}

		if report.StrippedSomething() && logger != nil {
			// Observability: the editionId and what kind of thing was
			// stripped — never the stripped content, the path, or the
			// surrounding document (constitution §8).
			logger.InfoContext(ctx, "reader content sanitised",
				"correlationId", correlationID,
				"editionId", editionID,
				"scriptsStripped", report.ScriptsStripped,
				"externalRefsStripped", report.ExternalRefsStripped,
				"svgStripped", report.SVGStripped,
				"styleAttrsStripped", report.StyleAttrsStripped,
			)
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Security-Policy", readerContentCSP)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=300")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}

// writeDomainError maps a *domain.Error to its category and writes the
// shared envelope; a non-domain error is an Internal.
func writeDomainError(w http.ResponseWriter, err error, correlationID string) {
	var derr *domain.Error
	if errors.As(err, &derr) {
		WriteError(w, derr.Category, derr.Message, correlationID)
		return
	}
	WriteError(w, domain.Internal, "unexpected error", correlationID)
}
