import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Input } from '../../components/Input/Input'
import { SourceCandidateList } from '../../components/SourceCandidateList/SourceCandidateList'
import { SourceStatusBadge } from '../../components/SourceStatusBadge/SourceStatusBadge'
import { Spinner } from '../../components/Spinner/Spinner'
import { useDiscoverImport } from '../../data/import'
import { useHealthCheckSource, useSource, useSourceCandidates } from '../../data/sources'
import { ApiError } from '../../data/http'
import { FOCUS_RING } from '../../lib/focusRing'
import { cx } from '../../lib/cx'
import { ChevronLeftIcon } from '../../components/Icon'

/**
 * SourceDetail / Browse view at /sources/:id.
 * Renders browsable and searchable candidate books for a specific source.
 */
export function SourceDetail() {
  const navigate = useNavigate()
  const params = useParams<{ id?: string; '*'?: string }>()
  const id =
    params.id ||
    (params['*'] ? params['*'].replace(/^(sources|source)\//, '').split('/')[0] : '') ||
    ''
  const {
    data: source,
    error: sourceError,
    isPending: isSourcePending,
    isError: isSourceError,
    refetch: refetchSource,
  } = useSource(id)
  const healthCheckMutation = useHealthCheckSource()
  const discoverMutation = useDiscoverImport()

  const [searchQuery, setSearchQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(searchQuery.trim())
    }, 300)
    return () => clearTimeout(timer)
  }, [searchQuery])

  const canSearch = Boolean(source?.capabilities?.canSearch)
  const activeQuery = canSearch ? debouncedQuery : undefined

  const {
    data: candidatesData,
    error: candidatesError,
    isPending: isCandidatesPending,
    isFetchingNextPage,
    hasNextPage,
    fetchNextPage,
    refetch: refetchCandidates,
  } = useSourceCandidates(id, { query: activeQuery })

  const candidates = candidatesData?.pages.flatMap((page) => page.items) ?? []

  if (isSourcePending) {
    return <Spinner label="Loading source" className="m-3xl" />
  }

  if (isSourceError || !source) {
    return (
      <div className="p-3xl">
        <ErrorState
          title="Could not load source"
          description={sourceError instanceof ApiError ? sourceError.message : 'Source not found.'}
          code={sourceError instanceof ApiError ? sourceError.code : undefined}
          correlationId={sourceError instanceof ApiError ? sourceError.correlationId : undefined}
          onRetry={() => void refetchSource()}
        />
      </div>
    )
  }

  const isLocal = source.kind === 'local-folder'
  const configDisplay = isLocal ? source.config.basePath : source.config.baseUrl

  return (
    <div className="flex flex-col gap-xl p-3xl content-container-wide">
      {/* Back link & Header */}
      <div className="flex flex-col gap-sm">
        <Link
          to="/sources"
          className={cx(
            'inline-flex items-center gap-xs self-start text-xs font-medium text-text-2 hover:text-text rounded-2xs py-4xs transition-colors',
            FOCUS_RING,
          )}
        >
          <ChevronLeftIcon className="size-3.5 shrink-0" aria-hidden="true" />
          <span>All sources</span>
        </Link>

        <div className="flex flex-wrap items-center justify-between gap-md mt-xs">
          <div className="flex items-center gap-md">
            <div className="source-avatar-lg bg-surface-3 flex items-center justify-center font-mono text-xs text-text-2 shrink-0 font-medium border border-border">
              {isLocal ? 'DIR' : 'OPDS'}
            </div>
            <div>
              <div className="flex items-center gap-sm">
                <h1 className="text-2xl font-semibold tracking-tight text-text">{source.label}</h1>
                <SourceStatusBadge
                  health={source.health}
                  onCheckAgain={() => healthCheckMutation.mutate(source.id)}
                  isChecking={
                    healthCheckMutation.isPending && healthCheckMutation.variables === source.id
                  }
                />
              </div>
              <p className="text-xs font-mono text-text-3 mt-4xs">
                {isLocal ? 'LOCAL FOLDER' : 'OPDS 2.0'} · {configDisplay}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-sm">
            <Button
              variant="primary"
              onClick={() => {
                discoverMutation.mutate(
                  { sourceId: source.id },
                  {
                    onSuccess: () => {
                      navigate(`/import?sourceId=${encodeURIComponent(source.id)}`)
                    },
                  },
                )
              }}
              disabled={discoverMutation.isPending}
            >
              {discoverMutation.isPending ? 'Discovering files...' : 'Import from this source'}
            </Button>
          </div>
        </div>

        {/* Prototype 4 Stat Cards */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-md mt-md">
          <div className="p-md rounded-md border border-border bg-surface shadow-xs">
            <div className="font-mono text-3xs text-text-3 uppercase tracking-wider">BOOKS INDEXED</div>
            <div className="text-xl font-medium text-text mt-4xs">
              {candidates.length > 0 ? `${candidates.length}` : '—'}
            </div>
          </div>
          <div className="p-md rounded-md border border-border bg-surface shadow-xs">
            <div className="font-mono text-3xs text-text-3 uppercase tracking-wider">LAST SYNC</div>
            <div className="text-xl font-medium text-text mt-4xs">
              {source.health.checkedAt
                ? new Date(source.health.checkedAt).toLocaleDateString()
                : 'Recent'}
            </div>
          </div>
          <div className="p-md rounded-md border border-border bg-surface shadow-xs">
            <div className="font-mono text-3xs text-text-3 uppercase tracking-wider">STORAGE</div>
            {/* NOTE(backend-gap): Source storage volume size is not currently exposed in API */}
            <div className="text-xl font-medium text-text mt-4xs">Connected</div>
          </div>
          <div className="p-md rounded-md border border-border bg-surface shadow-xs">
            <div className="font-mono text-3xs text-text-3 uppercase tracking-wider">AUTHENTICATION</div>
            <div className="text-sm font-medium text-text-2 mt-xs">
              {source.hasCredential ? 'Basic · stored' : 'None required'}
            </div>
          </div>
        </div>

        {discoverMutation.isError && (
          <div className="rounded-md border border-danger/30 bg-danger/10 p-sm text-sm text-danger mt-sm" role="alert">
            {discoverMutation.error instanceof ApiError
              ? discoverMutation.error.message
              : 'Failed to start import discovery.'}
          </div>
        )}
      </div>

      {/* Search Input (only when canSearch: true) */}
      {/* max-w-[28rem] not max-w-md: --spacing-md collides with Tailwind's max-w-md key */}
      {canSearch && (
        <div className="max-w-[28rem]">
          <Input
            label="Search this source"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search by title or author..."
            type="search"
          />
        </div>
      )}

      {/* Candidates List / Grid */}
      <SourceCandidateList
        candidates={candidates}
        hasNextPage={hasNextPage}
        isFetchingNextPage={isFetchingNextPage}
        fetchNextPage={fetchNextPage}
        isLoading={isCandidatesPending}
        error={candidatesError}
        onRetry={() => void refetchCandidates()}
        emptyMessage={
          debouncedQuery
            ? `No books found matching "${debouncedQuery}".`
            : 'No books found in this source.'
        }
      />
    </div>
  )
}
