/**
 * API & Interface Design Contracts for Alexandryn Backend Gaps.
 *
 * Grounded in the principles of `api-and-interface-design`:
 * 1. Contract First: Interfaces are established before backend implementation.
 * 2. Additive & Non-Breaking: Extended fields are optional or backward-compatible.
 * 3. Consistent Error Semantics: Structured error responses with machine-readable codes.
 * 4. Boundary Validation: Predictable schemas matching frontend consumption needs.
 */

// ---------------------------------------------------------------------------
// 1. Work Detail & Edition Extended Schema (GET /api/v1/works/:id)
// ---------------------------------------------------------------------------

/**
 * Expected schema extension for Work detail to satisfy rich reading metadata
 * in the UI (e.g. WorkDetail stat strip and editions view).
 */
export interface WorkDetailContract {
  id: string
  title: string
  subtitle?: string | null
  authors: string[]
  subjects: string[]
  originalLanguage?: string | null

  /**
   * NOTE(backend-gap): Page count of the authoritative or primary edition.
   * Nullable when page count cannot be determined from file metadata or Open Library.
   */
  pageCount?: number | null

  /**
   * Total number of editions known globally across Open Library and connected sources.
   */
  totalEditionsCount?: number

  /**
   * Cross-catalog identifiers for deduplication and external reference linking.
   */
  identifiers?: {
    openLibraryKey?: string
    isbn10?: string
    isbn13?: string
    asin?: string
    googleBooksId?: string
  }

  ownedEditions: EditionContract[]
  collections?: {
    id: string
    name: string
    addedAt?: string
  }[]
}

export interface EditionContract {
  id: string
  language: string
  isbn?: string | null
  publisher?: string | null
  publicationYear?: number | null
  addedAt?: string | null
  formats: string[]
  /** File size in bytes of the primary associated asset */
  fileSizeBytes?: number | null
  /** Measured page count of this specific edition */
  pageCount?: number | null
}

// ---------------------------------------------------------------------------
// 2. Source Detail & Storage Metrics (GET /api/v1/sources/:id)
// ---------------------------------------------------------------------------

/**
 * Expected schema extension for Source details to provide real-time volume
 * and quota metrics to the UI.
 */
export interface SourceStorageContract {
  /** Total bytes occupied by indexed books and associated assets in this source */
  usedBytes: number

  /** Total storage capacity of the underlying volume in bytes, or null if unknown/unlimited */
  totalBytes: number | null

  /** Available free space in bytes on the mounted volume */
  freeBytes?: number | null

  /** Filesystem path or mount point (for local-folder sources) */
  volumePath?: string | null

  /** Last measured timestamp in ISO 8601 format */
  measuredAt: string
}

export interface SourceSyncStatisticsContract {
  /** Number of works successfully indexed in the source's catalog */
  indexedBooksCount: number

  /** Duration of the last completed indexing job in milliseconds */
  lastSyncDurationMs?: number | null

  /** Number of indexing or file-parsing errors encountered during the last sync */
  errorsCount: number

  /** ISO 8601 timestamp when the last synchronization completed */
  completedAt?: string | null
}

export interface SourceDetailContract {
  id: string
  label: string
  kind: 'local-folder' | 'opds'
  config: {
    basePath?: string
    baseUrl?: string
  }
  hasCredential: boolean
  health: {
    status: 'reachable' | 'unreachable' | 'degraded'
    checkedAt: string | null
    detail?: string | null
  }
  capabilities: {
    canList: boolean
    canSearch: boolean
    canDownload: boolean
  }

  /** NOTE(backend-gap): Real-time storage volume usage and quota data */
  storage?: SourceStorageContract | null

  /** NOTE(backend-gap): Performance and indexing metrics */
  syncStatistics?: SourceSyncStatisticsContract | null
}

// ---------------------------------------------------------------------------
// 3. Alexandryn Standard Error Schema
// ---------------------------------------------------------------------------

export type AlexandrynErrorCode =
  | 'NOT_FOUND'
  | 'VALIDATION_ERROR'
  | 'UNAUTHORIZED'
  | 'FORBIDDEN'
  | 'SOURCE_UNREACHABLE'
  | 'SOURCE_TIMEOUT'
  | 'CONFLICT'
  | 'INTERNAL_ERROR'

/**
 * Standard structured API error returned on 4xx/5xx responses.
 */
export interface AlexandrynApiErrorResponse {
  error: {
    /** Machine-readable error code */
    code: AlexandrynErrorCode
    /** User-readable diagnostic explanation */
    message: string
    /** Unique request tracking identifier for debugging and support logs */
    correlationId?: string
    /** Field-level validation issues or structured diagnostic context */
    details?: Record<string, unknown>
  }
}
