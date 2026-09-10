package openlibrary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

const (
	defaultOpenLibraryBaseURL = "https://openlibrary.org"
	defaultCoversBaseURL      = "https://covers.openlibrary.org"
	maxMetadataResponseBytes  = 5 * 1024 * 1024  // 5 MiB (FR-9)
	maxCoverResponseBytes     = 10 * 1024 * 1024 // 10 MiB (backend-metadata-caching.md FR-6)
	defaultRequestTimeout     = 5 * time.Second  // 5 seconds (FR-9)

	// getWorkTotalTimeout bounds a whole GetWork call — one work fetch,
	// one editions fetch, and up to 20 author fetches, each otherwise
	// getting its own defaultRequestTimeout. Without this a slow Open
	// Library could hold the caller for 22 x 5s (#180).
	getWorkTotalTimeout = 15 * time.Second
)

// HTTPClient is the production implementation of Client.
type HTTPClient struct {
	baseURL        string
	coversBaseURL  string
	userAgent      string
	logger         *slog.Logger
	limiter        *RateLimiter
	httpClient     *http.Client
	getWorkTimeout time.Duration
}

// Option configures an HTTPClient.
type Option func(*HTTPClient)

// WithCoversBaseURL overrides the default Open Library covers base URL.
func WithCoversBaseURL(url string) Option {
	return func(c *HTTPClient) {
		if url != "" {
			c.coversBaseURL = strings.TrimRight(url, "/")
		}
	}
}

// WithGetWorkTimeout overrides the total time budget for one GetWork
// call (#180). Non-positive values are ignored.
func WithGetWorkTimeout(d time.Duration) Option {
	return func(c *HTTPClient) {
		if d > 0 {
			c.getWorkTimeout = d
		}
	}
}

// NewClient constructs a new HTTPClient.
func NewClient(baseURL, userAgent string, logger *slog.Logger, limiter *RateLimiter, httpClient *http.Client, opts ...Option) *HTTPClient {
	if baseURL == "" {
		baseURL = defaultOpenLibraryBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	if limiter == nil {
		limiter = DefaultRateLimiter(logger)
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:       defaultRequestTimeout,
			Transport:     guardedTransport(),
			CheckRedirect: capRedirects,
		}
	}

	c := &HTTPClient{
		baseURL:        baseURL,
		coversBaseURL:  defaultCoversBaseURL,
		userAgent:      userAgent,
		logger:         logger,
		limiter:        limiter,
		httpClient:     httpClient,
		getWorkTimeout: getWorkTotalTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

var _ Client = (*HTTPClient)(nil)

func sanitizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "[invalid-url]"
	}
	q := parsed.Query()
	if q.Has("q") {
		q.Set("q", "[redacted]")
		parsed.RawQuery = q.Encode()
	}
	return parsed.String()
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		sanitizedURL := sanitizeURL(urlErr.URL)
		return fmt.Sprintf("%s %q: %v", urlErr.Op, sanitizedURL, urlErr.Err)
	}
	return err.Error()
}

func (c *HTTPClient) doRequest(ctx context.Context, reqURL string, maxBytes int64) ([]byte, string, int, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, "", 0, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, "", 0, &domain.Error{Category: domain.Internal, Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json, image/jpeg, */*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Warn("Open Library outbound request failed",
				slog.String("url", sanitizeURL(reqURL)),
				slog.String("error", sanitizeError(err)),
			)
		}
		return nil, "", 0, &domain.Error{Category: domain.Unavailable, Message: "Open Library request failed or timed out"}
	}
	defer func() { _ = resp.Body.Close() }()

	contentType := resp.Header.Get("Content-Type")

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, contentType, resp.StatusCode, &domain.Error{Category: domain.NotFound, Message: "resource not found on Open Library"}
		}
		if c.logger != nil {
			c.logger.Warn("Open Library returned non-200 status", slog.String("url", sanitizeURL(reqURL)), slog.Int("status", resp.StatusCode))
		}
		return nil, contentType, resp.StatusCode, &domain.Error{Category: domain.Unavailable, Message: fmt.Sprintf("Open Library returned HTTP %d", resp.StatusCode)}
	}

	// Bounded body read
	lr := io.LimitReader(resp.Body, maxBytes+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, contentType, resp.StatusCode, &domain.Error{Category: domain.Unavailable, Message: "failed to read Open Library response body"}
	}
	if int64(len(body)) > maxBytes {
		if c.logger != nil {
			c.logger.Warn("Open Library response exceeded size limit", slog.String("url", sanitizeURL(reqURL)), slog.Int64("cap", maxBytes))
		}
		return nil, contentType, resp.StatusCode, &domain.Error{Category: domain.Unavailable, Message: "Open Library response exceeded maximum size limit"}
	}

	return body, contentType, resp.StatusCode, nil
}

// Search executes a search query against Open Library (FR-1).
func (c *HTTPClient) Search(ctx context.Context, q string, limit, offset int) (*NormalisedSearchResponse, error) {
	q = strings.TrimSpace(q)
	if q == "" || len([]rune(q)) > 200 {
		return nil, &domain.Error{Category: domain.InvalidInput, Message: "q: must be non-empty and <= 200 characters"}
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	params := url.Values{}
	params.Set("q", q)
	params.Set("fields", "key,title,author_name,author_key,first_publish_year,cover_i,edition_count,language")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(offset))

	searchURL := fmt.Sprintf("%s/search.json?%s", c.baseURL, params.Encode())
	body, _, _, err := c.doRequest(ctx, searchURL, maxMetadataResponseBytes)
	if err != nil {
		return nil, err
	}

	return NormaliseSearchResponse(body, limit, offset)
}

// GetWork fetches and normalises an Open Library work and its editions (FR-3).
func (c *HTTPClient) GetWork(ctx context.Context, openLibraryID string) (*DiscoverWorkDetail, error) {
	openLibraryID = CleanKey(openLibraryID)
	if !IsValidWorkKey(openLibraryID) {
		return nil, &domain.Error{Category: domain.InvalidInput, Message: fmt.Sprintf("invalid Open Library work key: %q", openLibraryID)}
	}

	// Bound the whole call (work + editions + author fan-out), not just
	// each request individually (#180).
	ctx, cancel := context.WithTimeout(ctx, c.getWorkTimeout)
	defer cancel()

	// 1. Fetch work record
	workURL := fmt.Sprintf("%s/works/%s.json", c.baseURL, openLibraryID)
	workBody, _, _, err := c.doRequest(ctx, workURL, maxMetadataResponseBytes)
	if err != nil {
		return nil, err
	}

	work, err := NormaliseWork(workBody)
	if err != nil {
		return nil, err
	}

	// 2. Fetch editions
	editionsURL := fmt.Sprintf("%s/works/%s/editions.json?limit=50", c.baseURL, openLibraryID)
	editionsBody, _, _, err := c.doRequest(ctx, editionsURL, maxMetadataResponseBytes)
	var editions []NormalisedEdition
	if err == nil {
		if normEds, normErr := NormaliseEditions(editionsBody); normErr == nil {
			editions = normEds
		}
	}
	if editions == nil {
		editions = []NormalisedEdition{}
	}

	// 3. Resolve authors (capped at first 20 distinct authors per FR-3)
	authorKeys := ExtractAuthorKeys(workBody)
	if len(authorKeys) > 20 {
		authorKeys = authorKeys[:20]
	}

	var authors []NormalisedAuthor
	for _, aKey := range authorKeys {
		authorURL := fmt.Sprintf("%s/authors/%s.json", c.baseURL, aKey)
		aBody, _, _, aErr := c.doRequest(ctx, authorURL, maxMetadataResponseBytes)
		if aErr != nil {
			// FR-3 & FR-9: single author fetch failure degrades to omission
			continue
		}
		if normAuthor, normErr := NormaliseAuthor(aKey, aBody); normErr == nil {
			authors = append(authors, *normAuthor)
		}
	}
	if authors == nil {
		authors = []NormalisedAuthor{}
	}
	work.Authors = authors

	return &DiscoverWorkDetail{
		Work:     *work,
		Editions: editions,
	}, nil
}

// FetchCover downloads a cover image binary from Open Library Covers API (backend-metadata-caching.md FR-6).
func (c *HTTPClient) FetchCover(ctx context.Context, coverID int64) ([]byte, string, error) {
	if coverID <= 0 {
		return nil, "", &domain.Error{Category: domain.InvalidInput, Message: "coverId must be a positive integer"}
	}

	coverURL := fmt.Sprintf("%s/b/id/%d-M.jpg?default=false", c.coversBaseURL, coverID)
	body, contentType, _, err := c.doRequest(ctx, coverURL, maxCoverResponseBytes)
	if err != nil {
		return nil, "", err
	}

	return body, contentType, nil
}
