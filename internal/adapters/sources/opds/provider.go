// Package opds is the OPDS source provider
// (backend-source-adapter.md FR-4..FR-11): it fetches and normalises
// OPDS 1.2 (Atom) and OPDS 2.0 (JSON) feeds, with redirect-following
// disabled entirely and every source-supplied URL origin-checked
// against the configured baseUrl before it is fetched.
package opds

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Config constructs a Provider.
type Config struct {
	SourceID      string
	BaseURL       string
	Credential    sources.Credential
	HasCredential bool
	// SearchTemplate is the origin-validated templated search URL
	// captured at the last Probe (FR-11 — validated once, reused as-is).
	// Empty when the source advertises no usable search.
	SearchTemplate string
	Semaphore      *sources.Semaphore
	Codec          *sources.CursorCodec
	Logger         *slog.Logger
	// AllowPrivateAddresses permits the outbound client to reach loopback
	// and RFC 1918 / ULA addresses (SOURCE_ALLOW_PRIVATE_ADDRESSES). Off
	// by default: a source base URL is attacker-chosen input (§4), so the
	// dialer blocks non-public targets and defends DNS rebinding.
	// Link-local, CGNAT, and multicast are blocked regardless.
	AllowPrivateAddresses bool
}

// Provider implements sources.Provider for an OPDS catalog.
type Provider struct {
	sourceID       string
	baseURL        string
	client         *httpClient
	codec          *sources.CursorCodec
	logger         *slog.Logger
	searchTemplate string
}

var _ sources.Provider = (*Provider)(nil)

// New builds a Provider. It does no I/O.
func New(cfg Config) *Provider {
	return &Provider{
		sourceID:       cfg.SourceID,
		baseURL:        cfg.BaseURL,
		client:         newHTTPClient(cfg.Semaphore, cfg.Credential, cfg.HasCredential, cfg.AllowPrivateAddresses),
		codec:          cfg.Codec,
		logger:         cfg.Logger,
		searchTemplate: cfg.SearchTemplate,
	}
}

// Probe fetches the root feed, classifies reachability into FR-6's
// closed vocabulary, and re-detects capabilities and the search link
// (FR-5, FR-11). It never returns an error.
func (p *Provider) Probe(ctx context.Context) sources.ProbeResult {
	body, ferr := p.client.get(ctx, p.baseURL)
	if ferr != nil {
		return sources.ProbeResult{Status: sources.HealthUnreachable, Detail: ferr.detail}
	}

	pf, err := parseFeed(body)
	if err != nil {
		return sources.ProbeResult{Status: sources.HealthUnreachable, Detail: sources.DetailUnparseable}
	}

	caps := domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true}
	searchURL := p.detectSearch(ctx, pf)
	if searchURL != "" {
		caps.CanSearch = true
	}

	return sources.ProbeResult{
		Status:        sources.HealthReachable,
		Capabilities:  caps,
		SearchLinkURL: searchURL,
	}
}

// detectSearch resolves the feed's rel="search" link to an
// origin-validated templated query URL, or "" when the source
// advertises none or advertises an off-origin one (FR-11 — an unusable,
// untrusted link is discarded, not stored).
func (p *Provider) detectSearch(ctx context.Context, pf parsedFeed) string {
	if pf.searchHref == "" {
		return ""
	}
	resolved := resolveSameOrigin(p.baseURL, pf.searchHref)
	if resolved == "" {
		return ""
	}

	if !pf.searchIsDescriptionDoc {
		// OPDS 2.0: the link is the (templated) query URL itself.
		return resolved
	}

	// OPDS 1.2: fetch the OpenSearch description document (same-origin,
	// already validated) and extract its template.
	body, ferr := p.client.get(ctx, resolved)
	if ferr != nil {
		return ""
	}
	template := parseOpenSearchTemplate(body)
	if template == "" {
		return ""
	}
	return resolveSameOrigin(p.baseURL, template)
}

// List returns one page of the catalog's publications (FR-7).
func (p *Provider) List(ctx context.Context, cursor string, limit int) (sources.CandidatePage, error) {
	_ = sources.ClampLimit(limit) // page size is the feed's own; limit bounds nothing extra here

	fetchURL := p.baseURL
	if cursor != "" {
		c, err := p.codec.Decode(cursor, p.sourceID)
		if err != nil {
			return sources.CandidatePage{}, err
		}
		pos, err := c.ForOPDSPosition(p.baseURL)
		if err != nil {
			return sources.CandidatePage{}, err
		}
		if pos == "" {
			return sources.CandidatePage{}, nil
		}
		fetchURL = pos
	}

	body, ferr := p.client.get(ctx, fetchURL)
	if ferr != nil {
		return sources.CandidatePage{}, ferr.asDomain()
	}
	pf, err := parseFeed(body)
	if err != nil {
		return sources.CandidatePage{}, &domain.Error{Category: domain.Unavailable, Message: "source returned an unreadable response"}
	}
	return normalise(pf, p.baseURL, p.codec, p.sourceID), nil
}

// Search runs a query against the catalog's advertised search endpoint
// (FR-8). It returns Conflict when the source has no usable search.
func (p *Provider) Search(ctx context.Context, q, cursor string, limit int) (sources.CandidatePage, error) {
	_ = sources.ClampLimit(limit)

	if p.searchTemplate == "" {
		return sources.CandidatePage{}, &domain.Error{Category: domain.Conflict, Message: "this source does not support search"}
	}

	fetchURL := expandSearchTemplate(p.searchTemplate, q)
	if cursor != "" {
		c, err := p.codec.Decode(cursor, p.sourceID)
		if err != nil {
			return sources.CandidatePage{}, err
		}
		pos, err := c.ForOPDSPosition(p.baseURL)
		if err != nil {
			return sources.CandidatePage{}, err
		}
		if pos == "" {
			return sources.CandidatePage{}, nil
		}
		fetchURL = pos
	}

	// The expanded template must still be same-origin — a template
	// captured at probe time was validated, but expansion cannot change
	// the origin, so this is belt-and-braces before the fetch (FR-11).
	if !sources.SameOrigin(p.baseURL, fetchURL) {
		return sources.CandidatePage{}, &domain.Error{Category: domain.Unavailable, Message: "source is unavailable right now"}
	}

	body, ferr := p.client.get(ctx, fetchURL)
	if ferr != nil {
		return sources.CandidatePage{}, ferr.asDomain()
	}
	pf, err := parseFeed(body)
	if err != nil {
		return sources.CandidatePage{}, &domain.Error{Category: domain.Unavailable, Message: "source returned an unreadable response"}
	}
	return normalise(pf, p.baseURL, p.codec, p.sourceID), nil
}

// Resolve fetches the bytes behind an acquisition reference (FR-15,
// phase 10). The reference id is the acquisition href; it is
// origin-checked against baseUrl before any request.
func (p *Provider) Resolve(ctx context.Context, ref domain.FileReference) (io.ReadCloser, error) {
	target := resolveSameOrigin(p.baseURL, ref.ReferenceID)
	if target == "" {
		return nil, &domain.Error{Category: domain.InvalidInput, Message: "file reference is not valid for this source"}
	}
	body, ferr := p.client.get(ctx, target)
	if ferr != nil {
		return nil, ferr.asDomain()
	}
	return io.NopCloser(strings.NewReader(string(body))), nil
}

// expandSearchTemplate substitutes q into an OpenSearch / RFC 6570
// query template. When the template carries no recognisable macro, q is
// appended as a `q` query parameter.
func expandSearchTemplate(template, q string) string {
	esc := url.QueryEscape(q)
	if strings.Contains(template, "{searchTerms}") {
		return strings.ReplaceAll(template, "{searchTerms}", esc)
	}
	if strings.Contains(template, "{?query}") {
		return strings.ReplaceAll(template, "{?query}", "?query="+esc)
	}
	if strings.Contains(template, "{query}") {
		return strings.ReplaceAll(template, "{query}", esc)
	}
	sep := "?"
	if strings.Contains(template, "?") {
		sep = "&"
	}
	return template + sep + "q=" + esc
}

// parseFeed picks the OPDS 1.2 or 2.0 parser by content shape.
func parseFeed(body []byte) (parsedFeed, error) {
	if looksLikeXML(body) {
		return parseAtom(body)
	}
	return parseOPDS2(body)
}
