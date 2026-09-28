import { Button } from '../../components/Button'
import { Spinner } from '../../components/Spinner/Spinner'
import { useDiagnostics } from '../../data/diagnostics'

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  const kibi = bytes / 1024
  if (kibi < 1024) return `${kibi.toFixed(1)} KiB`
  const mebi = kibi / 1024
  if (mebi < 1024) return `${mebi.toFixed(1)} MiB`
  const gibi = mebi / 1024
  return `${gibi.toFixed(2)} GiB`
}

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  if (m < 60) return `${m}m ${s}s`
  const h = Math.floor(m / 60)
  const remM = m % 60
  if (h < 24) return `${h}h ${remM}m`
  const d = Math.floor(h / 24)
  const remH = h % 24
  return `${d}d ${remH}h ${remM}m`
}

export function DiagnosticsScreen() {
  const { data, isLoading, isError, error, refetch, isFetching } = useDiagnostics()

  return (
    <div className="p-xl max-w-[48rem] mx-auto flex flex-col gap-lg">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-ui font-medium text-text">System Diagnostics</h1>
          <p className="text-sm text-text-2 mt-4xs max-w-[42rem]">
            Live server runtime metrics, memory utilization, and build version.
          </p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => void refetch()}
          disabled={isFetching}
        >
          {isFetching ? 'Refreshing…' : 'Refresh'}
        </Button>
      </div>

      {isLoading && (
        <div className="flex flex-col items-center justify-center p-3xl gap-md">
          <Spinner label="Loading diagnostics" />
          <p className="text-xs text-text-2">Collecting system metrics…</p>
        </div>
      )}

      {isError && (
        <div className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error flex items-center justify-between">
          <span>{error instanceof Error ? error.message : 'Failed to load diagnostics'}</span>
          <Button variant="secondary" size="sm" onClick={() => void refetch()}>
            Retry
          </Button>
        </div>
      )}

      {data && (
        <>
          {/* Server Uptime & Status */}
          <section
            aria-labelledby="uptime-heading"
            className="rounded-md border border-border bg-surface p-lg flex flex-col gap-md shadow-sm"
          >
            <div className="flex items-center justify-between">
              <h2 id="uptime-heading" className="text-lg font-medium text-text">
                Service Status
              </h2>
              <div className="flex items-center gap-xs">
                <span className="size-2 rounded-full bg-success" />
                <span className="text-xs font-mono font-medium text-success uppercase">Healthy</span>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-md">
              <div className="flex flex-col gap-4xs">
                <span className="text-xs uppercase font-medium tracking-wide text-text-3">
                  Server Uptime
                </span>
                <span className="font-mono text-sm text-text font-medium">
                  {formatUptime(data.uptime_seconds)}
                </span>
              </div>

              <div className="flex flex-col gap-4xs">
                <span className="text-xs uppercase font-medium tracking-wide text-text-3">
                  Active Goroutines
                </span>
                <span className="font-mono text-sm text-text font-medium">
                  {data.runtime.goroutines.toLocaleString()}
                </span>
              </div>
            </div>
          </section>

          {/* Go Runtime & Memory */}
          <section
            aria-labelledby="runtime-heading"
            className="rounded-md border border-border bg-surface p-lg flex flex-col gap-md shadow-sm"
          >
            <h2 id="runtime-heading" className="text-lg font-medium text-text">
              Go Runtime & Memory
            </h2>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-md">
              <div className="flex flex-col gap-4xs">
                <span className="text-xs uppercase font-medium tracking-wide text-text-3">
                  Heap Allocated
                </span>
                <span className="font-mono text-sm text-text font-medium">
                  {formatBytes(data.runtime.heap_alloc_bytes)}
                </span>
              </div>

              <div className="flex flex-col gap-4xs">
                <span className="text-xs uppercase font-medium tracking-wide text-text-3">
                  Cumulative Allocated
                </span>
                <span className="font-mono text-sm text-text font-medium">
                  {formatBytes(data.runtime.total_alloc_bytes)}
                </span>
              </div>

              <div className="flex flex-col gap-4xs">
                <span className="text-xs uppercase font-medium tracking-wide text-text-3">
                  Garbage Collector Cycles
                </span>
                <span className="font-mono text-sm text-text font-medium">
                  {data.runtime.gc_cycles.toLocaleString()}
                </span>
              </div>
            </div>
          </section>

          {/* Build & Version Information */}
          <section
            aria-labelledby="version-heading"
            className="rounded-md border border-border bg-surface p-lg flex flex-col gap-md shadow-sm"
          >
            <h2 id="version-heading" className="text-lg font-medium text-text">
              Version & Build
            </h2>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-md">
              <div className="flex flex-col gap-4xs">
                <span className="text-xs uppercase font-medium tracking-wide text-text-3">
                  Git Commit SHA
                </span>
                <span className="font-mono text-xs text-text font-medium select-all">
                  {data.version.commit || 'development'}
                </span>
              </div>

              <div className="flex flex-col gap-4xs">
                <span className="text-xs uppercase font-medium tracking-wide text-text-3">
                  Build Timestamp
                </span>
                <span className="font-mono text-xs text-text font-medium">
                  {data.version.build_time || 'unspecified'}
                </span>
              </div>
            </div>
          </section>
        </>
      )}
    </div>
  )
}
