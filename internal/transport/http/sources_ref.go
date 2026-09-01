package http

import (
	"sync/atomic"

	"github.com/Alexandryn/alexandryn/internal/adapters/crypto"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// SourceCrypto bundles the credential encryptor and the cursor codec —
// both derived from the same key file (backend-source-adapter.md FR-13,
// FR-7), set together once persistence is ready.
type SourceCrypto struct {
	Encryptor *crypto.Service
	Codec     *sources.CursorCodec
}

// The phase-08 source surface needs three things that only exist once
// the pool is up: the full-row repository, the atomic removal service
// (domain-source.md FR-6), and the credential/cursor crypto. They ride
// on PoolRef alongside metadataCache/coverCache for the same reason
// those do — the router builds handlers before persistence is ready, so
// the handlers reach these lazily.
type sourceRefs struct {
	records atomic.Pointer[postgres.SourceRecordRepository]
	removal atomic.Pointer[domain.SourceRemovalService]
	crypto  atomic.Pointer[SourceCrypto]
}

// SetSourceRecordRepository stores the source-record repository.
func (r *PoolRef) SetSourceRecordRepository(repo *postgres.SourceRecordRepository) {
	r.sources.records.Store(repo)
}

// GetSourceRecordRepository returns the source-record repository, if set.
func (r *PoolRef) GetSourceRecordRepository() (*postgres.SourceRecordRepository, bool) {
	v := r.sources.records.Load()
	return v, v != nil
}

// SetSourceRemovalService stores the atomic source-removal service.
func (r *PoolRef) SetSourceRemovalService(svc *domain.SourceRemovalService) {
	r.sources.removal.Store(svc)
}

// GetSourceRemovalService returns the source-removal service, if set.
func (r *PoolRef) GetSourceRemovalService() (*domain.SourceRemovalService, bool) {
	v := r.sources.removal.Load()
	return v, v != nil
}

// SetSourceCrypto stores the credential encryptor and cursor codec.
func (r *PoolRef) SetSourceCrypto(sc SourceCrypto) {
	r.sources.crypto.Store(&sc)
}

// GetSourceCrypto returns the source crypto bundle, if set.
func (r *PoolRef) GetSourceCrypto() (SourceCrypto, bool) {
	v := r.sources.crypto.Load()
	if v == nil {
		return SourceCrypto{}, false
	}
	return *v, true
}
