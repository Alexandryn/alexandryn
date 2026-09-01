// Package sources is phase 08's source-adapter layer
// (backend-source-adapter.md). It turns a raw OPDS feed or a local
// directory listing into candidates this project's domain and UI can
// trust, treating every response and filename as hostile input
// (constitution §4).
//
// Domain boundary (constitution §3): no OPDS or filesystem type leaves
// this package. SourceCandidate is an adapter-owned DTO, never a
// domain-source.md SourceOffering (FR-10) — nothing here constructs one.
// The single domain type this package builds is domain.FileReference,
// which FR-4 designed to need no Edition.
package sources

import (
	"context"
	"io"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Kind is the closed provider vocabulary (backend-source-adapter.md
// FR-1). domain-source.md FR-1 leaves the value set to phase 08.
type Kind string

const (
	KindLocalFolder Kind = "local-folder"
	KindOPDS        Kind = "opds"
)

// ValidKind reports whether s is one of the two supported kinds.
func ValidKind(s string) bool {
	return Kind(s) == KindLocalFolder || Kind(s) == KindOPDS
}

// HealthStatus is a source's last reachability observation.
type HealthStatus string

const (
	HealthUnknown     HealthStatus = "unknown"
	HealthReachable   HealthStatus = "reachable"
	HealthUnreachable HealthStatus = "unreachable"
)

// Health-check detail categories (backend-source-adapter.md FR-6). A
// closed vocabulary — never a raw error string or response body, which
// could leak an echoed Authorization header or a verbose error page.
const (
	DetailTimeout            = "timeout"
	DetailConnectionRefused  = "connection-refused"
	DetailHTTP4xx            = "http-4xx"
	DetailHTTP5xx            = "http-5xx"
	DetailHTTP3xxUnsupported = "http-3xx-unsupported"
	DetailUnparseable        = "unparseable-response"
	DetailPathNotFound       = "path-not-found"
	DetailPathNotReadable    = "path-not-readable"
	DetailAuthRejected       = "auth-rejected"
)

// SourceCandidate is the shared normalised shape for browse (FR-7) and
// search (FR-8). It carries only what FR-9 permits — no raw Atom element
// or OPDS 2.0 field name reaches this struct or anything past this
// package.
type SourceCandidate struct {
	Title         string
	Author        *string
	FileReference domain.FileReference
	CoverURL      *string
}

// CandidatePage is one page of candidates plus an opaque continuation
// token (nil on the last page). The token is server-signed and
// re-validated on the way back in (FR-7, FR-11).
type CandidatePage struct {
	Items      []SourceCandidate
	NextCursor *string
}

// ProbeResult is a health check's outcome (FR-5, FR-6): reachability
// plus the capabilities and search link re-detected on every probe.
type ProbeResult struct {
	Status HealthStatus
	// Detail is one of the Detail* constants when Status is
	// HealthUnreachable, empty otherwise.
	Detail       string
	Capabilities domain.SourceCapabilities
	// SearchLinkURL is the origin-validated search endpoint discovered
	// on the root feed (FR-11), empty when the source advertises none or
	// advertises an off-origin one.
	SearchLinkURL string
}

// Provider is the per-source capability surface (FR-15). Both the HTTP
// handlers and any in-process caller (backend-import-pipeline.md, phase
// 10) share one implementation — List and Resolve are callable directly,
// with FR-7's validation and FR-11's SSRF/redirect protections applying
// either way.
type Provider interface {
	// Probe runs a lightweight reachability check and re-detects
	// capabilities. It never returns an error — an unreachable source is
	// a ProbeResult with Status HealthUnreachable and a Detail category.
	Probe(ctx context.Context) ProbeResult

	// List returns one page of the source's contents. cursor is the
	// opaque token from a previous call, or "" for the first page.
	List(ctx context.Context, cursor string, limit int) (CandidatePage, error)

	// Search returns one page of results for q. It returns a
	// *domain.Error with category Conflict when the source's detected
	// CanSearch is false (FR-8).
	Search(ctx context.Context, q, cursor string, limit int) (CandidatePage, error)

	// Resolve opens the bytes behind a FileReference (FR-15, for phase
	// 10). No HTTP endpoint exposes this in phase 08.
	Resolve(ctx context.Context, ref domain.FileReference) (io.ReadCloser, error)
}

// Page-size bounds shared by browse and search (FR-7), matching
// backend-metadata-adapter.md FR-1's own numbers.
const (
	DefaultLimit = 20
	MaxLimit     = 50
	MaxQueryLen  = 200
)

// ClampLimit returns limit when it is in [1, MaxLimit] and DefaultLimit
// otherwise. Transport rejects an out-of-range limit before calling a
// provider; this is the provider-side floor for direct callers.
func ClampLimit(limit int) int {
	if limit < 1 || limit > MaxLimit {
		return DefaultLimit
	}
	return limit
}
