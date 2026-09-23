package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/reader/content"
)

// readerContentPerRequestTimeout bounds every content request regardless
// of how long the cache holds a book open.
const readerContentPerRequestTimeout = 30 * time.Second

// readerContentCSP is the response policy a sandboxed client leans on as
// a second, independent layer over its own iframe sandbox.
// frame-ancestors 'self': the reader UI
// frames this content in a same-origin sandboxed <iframe src=…>,
// which the app-document security-headers
// middleware's X-Frame-Options: DENY would otherwise block — this
// endpoint owns its own framing policy and allows exactly
// same-origin framing.
const readerContentCSP = "default-src 'self'; script-src 'none'; object-src 'none'; frame-ancestors 'self'"

// ReaderContentHandler serves one sanitised entry from inside an owned
// Edition's EPUB: GET /api/v1/library/editions/{editionId}/reader/content/{path...}.
// The content cache is constructed once
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
		// of a library that owns this edition. The auth middleware
		// has already validated X-Library-Id against the token; here we
		// confirm the edition is in that library. This gate fails closed:
		// if the ownership checker is not wired, the content is not served.
		deps, ready := poolRef.GetReadingAPI()
		if !ready {
			WriteError(w, domain.Unavailable, "the reader is not ready yet", correlationID)
			return
		}
		if UserFromContext(r.Context()) == nil {
			WriteError(w, domain.Unauthorized, "authentication is required", correlationID)
			return
		}
		if !assertEditionInLibrary(deps, r, w, domain.EditionID(editionID), ActiveLibraryFromContext(r.Context()), correlationID) {
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
			// surrounding document.
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
		// Override the app-document middleware's X-Frame-Options: DENY —
		// this content is framed same-origin by the reader UI's sandboxed
		// iframe. SAMEORIGIN still refuses a cross-origin framer.
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Cache-Control", "private, max-age=300")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}

// ReaderContentCookieName is the cookie that carries a reader-content
// grant (auth.TokenTypeReaderContent) to the content route.
const ReaderContentCookieName = "alx_rc"

// readerEditionsPrefix is the path prefix shared by the reader content
// route and the grant cookie's Path.
const readerEditionsPrefix = "/api/v1/library/editions/"

// readerContentCookiePath scopes a grant cookie to one edition's content
// route, so the browser never sends it anywhere else.
func readerContentCookiePath(editionID domain.EditionID) string {
	return readerEditionsPrefix + string(editionID) + "/reader/content/"
}

// ReaderSessionHandler issues the reader-content grant cookie for one
// owned Edition: POST /api/v1/library/editions/{editionId}/reader/session.
// The reader calls it (Bearer-authenticated, like every other API call)
// before pointing its <iframe> at the content route, and again before the
// grant expires. The grant never appears in a URL, a response body, or a
// log line: it is an HttpOnly, SameSite=Strict cookie whose Path is that
// edition's content route alone.
func ReaderSessionHandler(poolRef *PoolRef, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())

		editionID := domain.EditionID(r.PathValue("editionId"))
		if editionID == "" || strings.Contains(string(editionID), "/") {
			WriteError(w, domain.InvalidInput, "editionId is required", correlationID)
			return
		}
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "authentication is required", correlationID)
			return
		}
		authAPI, ok := poolRef.GetAuthAPI()
		if !ok || authAPI.ReaderGrants == nil {
			WriteError(w, domain.Unavailable, "the reader is not ready yet", correlationID)
			return
		}
		deps, ready := poolRef.GetReadingAPI()
		if !ready {
			WriteError(w, domain.Unavailable, "the reader is not ready yet", correlationID)
			return
		}
		libraryID := ActiveLibraryFromContext(r.Context())
		if !assertEditionInLibrary(deps, r, w, editionID, libraryID, correlationID) {
			return
		}

		token, expiresAt, err := authAPI.ReaderGrants.Sign(user.UserID, libraryID, editionID, now())
		if err != nil {
			WriteError(w, domain.Internal, "unexpected error", correlationID)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     ReaderContentCookieName,
			Value:    token,
			Path:     readerContentCookiePath(editionID),
			Expires:  expiresAt,
			MaxAge:   int(auth.ReaderContentGrantTTL / time.Second),
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
		})
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
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
