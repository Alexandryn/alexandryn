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

/**
 * SourceDetail / Browse view at /sources/:id (frontend-source-management.md FR-6).
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
    <div className="flex flex-col gap-xl p-3xl">
      {/* Back link & Header */}
      <div className="flex flex-col gap-sm">
        <Link
          to="/sources"
          className={cx(
            'self-start text-xs font-medium text-text-2 hover:text-text rounded py-4xs',
            FOCUS_RING,
          )}
        >
          ← All sources
        </Link>

        <div className="flex flex-wrap items-center justify-between gap-md">
          <div>
            <div className="flex items-center gap-xs">
              <span className="text-2xl" aria-hidden="true">
                {isLocal ? '📁' : '🌐'}
              </span>
              <h1 className="text-3xl font-medium tracking-1 text-text">{source.label}</h1>
            </div>
            <p className="text-xs font-mono text-text-3 mt-4xs">{configDisplay}</p>
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
            <SourceStatusBadge
              health={source.health}
              onCheckAgain={() => healthCheckMutation.mutate(source.id)}
              isChecking={
                healthCheckMutation.isPending && healthCheckMutation.variables === source.id
              }
            />
          </div>
        </div>

        {discoverMutation.isError && (
          <div className="rounded-md border border-danger/30 bg-danger/10 p-sm text-sm text-danger" role="alert">
            {discoverMutation.error instanceof ApiError
              ? discoverMutation.error.message
              : 'Failed to start import discovery.'}
          </div>
        )}
      </div>

      {/* Search Input (only when canSearch: true) */}
      {/* max-w-[28rem] not max-w-md: --spacing-md collides with Tailwind's max-w-md key (audit 0017 A-17-08) */}
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
