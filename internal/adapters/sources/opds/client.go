package opds

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

const (
	// maxResponseBytes caps every OPDS response body
	// (backend-source-adapter.md FR-6).
	maxResponseBytes = 5 << 20 // 5 MiB
	// requestTimeout bounds every outbound call (FR-6).
	requestTimeout = 5 * time.Second
	acceptHeader   = "application/atom+xml, application/opds+json, application/json;q=0.9, */*;q=0.1"
)

// fetchError carries a health-check detail category (FR-6) alongside the
// error, so Probe can classify a failure without inspecting a raw
// message and List/Search can map it to a domain category.
type fetchError struct {
	detail string
	msg    string
}

func (e *fetchError) Error() string { return e.msg }

// asDomain maps a fetchError to the *domain.Error a browse/search call
// returns — every failure category collapses to Unavailable there,
// since "this call cannot proceed" is functionally identical to the
// caller whether the source is down, redirecting, or rejecting auth
// (FR-13). The distinguishing detail is surfaced by the next health
// check, not here.
func (e *fetchError) asDomain() *domain.Error {
	return &domain.Error{Category: domain.Unavailable, Message: "source is unavailable right now"}
}

// httpClient is the shared outbound client for one source. Redirects are
// disabled entirely (FR-11): a 3xx is that call's own failure, never
// followed, closing the gap an origin check on the initial URL alone
// would leave open.
type httpClient struct {
	hc      *http.Client
	sem     *sources.Semaphore
	cred    sources.Credential
	hasAuth bool
}

func newHTTPClient(sem *sources.Semaphore, cred sources.Credential, hasAuth, allowPrivate bool) *httpClient {
	return &httpClient{
		hc: &http.Client{
			Timeout:   requestTimeout,
			Transport: sources.GuardedTransport(allowPrivate),
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		sem:     sem,
		cred:    cred,
		hasAuth: hasAuth,
	}
}

// get fetches rawURL. The caller MUST have already validated rawURL as
// same-origin with the source's configured baseUrl (FR-11) — this
// method does not re-check. It returns the response body on a 2xx, or a
// *fetchError classified into FR-6's closed vocabulary.
func (c *httpClient) get(ctx context.Context, rawURL string) ([]byte, *fetchError) {
	if c.sem != nil && !c.sem.TryAcquire() {
		return nil, &fetchError{detail: sources.DetailTimeout, msg: "outbound concurrency cap reached"}
	}
	if c.sem != nil {
		defer c.sem.Release()
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &fetchError{detail: sources.DetailUnparseable, msg: "could not build request"}
	}
	req.Header.Set("Accept", acceptHeader)
	if c.hasAuth && !c.cred.IsZero() {
		// FR-4: the credential is sent on every request, unconditionally
		// — never "try without auth first".
		user, pass := c.cred.Reveal()
		req.SetBasicAuth(user, pass)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, classifyTransportError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, &fetchError{detail: sources.DetailHTTP3xxUnsupported, msg: "source responded with an unsupported redirect"}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &fetchError{detail: sources.DetailAuthRejected, msg: "source rejected the credential"}
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, &fetchError{detail: sources.DetailHTTP4xx, msg: "source returned a client error"}
	}
	if resp.StatusCode >= 500 {
		return nil, &fetchError{detail: sources.DetailHTTP5xx, msg: "source returned a server error"}
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if readErr != nil {
		return nil, &fetchError{detail: sources.DetailUnparseable, msg: "could not read the response body"}
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, &fetchError{detail: sources.DetailUnparseable, msg: "response exceeded the size limit"}
	}
	return body, nil
}

func classifyTransportError(err error) *fetchError {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &fetchError{detail: sources.DetailTimeout, msg: "source timed out"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &fetchError{detail: sources.DetailTimeout, msg: "source timed out"}
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" {
			return &fetchError{detail: sources.DetailConnectionRefused, msg: "could not connect to the source"}
		}
	}
	if errors.Is(err, context.Canceled) {
		return &fetchError{detail: sources.DetailTimeout, msg: "request cancelled"}
	}
	return &fetchError{detail: sources.DetailConnectionRefused, msg: "could not reach the source"}
}
