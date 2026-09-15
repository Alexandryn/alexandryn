package contracttest_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil/contracttest"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// TestSecurityAnnotationsMatchAuthMiddleware proves that every operation's
// `security` override in api/openapi.yaml agrees with whether
// AuthMiddleware actually requires a bearer token for that path.
//
// ValidateResponse (contract.go) deliberately bypasses authentication via
// openapi3filter.NoopAuthenticationFunc — response-shape validation and
// auth enforcement are different concerns. That leaves the spec's
// `security: []` annotations themselves unverified against real server
// behavior; this test closes that specific gap by comparing the two
// authorities directly, independent of any HTTP round trip.
func TestSecurityAnnotationsMatchAuthMiddleware(t *testing.T) {
	v := contracttest.New(t)
	doc := v.Doc()

	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			specPublic := op.Security != nil && len(*op.Security) == 0
			realPublic := transporthttp.IsPublicPath(path)
			if specPublic != realPublic {
				t.Errorf(
					"%s %s: spec declares security:[] (public=%v) but AuthMiddleware.IsPublicPath reports public=%v — "+
						"the contract and the running server disagree on whether this route requires authentication",
					method, path, specPublic, realPublic,
				)
			}
		}
	}
}
