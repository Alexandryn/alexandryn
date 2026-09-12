// The one place the app calls fetch: every server-derived value goes through
// a TanStack Query hook whose queryFn calls one of these helpers, never fetch
// inline in a component.

export interface ApiErrorBody {
  code: string
  message: string
  correlationId: string
}

/** A non-2xx API response, carrying standard error shape (code, message, correlationId). */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly correlationId: string | undefined

  constructor(status: number, body: Partial<ApiErrorBody> | null) {
    super(body?.message ?? `Request failed (${status})`)
    this.name = 'ApiError'
    this.status = status
    this.code = body?.code ?? 'unknown'
    this.correlationId = body?.correlationId
  }
}

/**
 * Resolves `path` against the current origin. The Go server serves this
 * app, so its origin is ours; an absolute base also lets fetch work under
 * jsdom, where a bare "/api/…" has no base to resolve against.
 */
export function apiUrl(path: string): string {
  return new URL(path, window.location.origin).toString()
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as Partial<ApiErrorBody> | null
    throw new ApiError(res.status, body)
  }
  if (res.status === 204 || res.status === 205) {
    return undefined as unknown as T
  }
  const text = await res.text()
  if (text === '') {
    // An empty body is a legitimate void result only for a no-content
    // response: an explicit `Content-Length: 0`, or a non-JSON content
    // type (a bare `w.WriteHeader(200)` with no body). An empty body
    // from a JSON resource endpoint is a malformed response and must
    // surface as an error, not a silent `undefined` that a caller then
    // reads a field off.
    const contentType = res.headers.get('Content-Type') ?? ''
    if (res.headers.get('Content-Length') === '0' || !contentType.includes('json')) {
      return undefined as unknown as T
    }
  }
  return JSON.parse(text) as T
}

/** Extra request headers — used by the reader for `X-Device-Id`, etc. */
export type ExtraHeaders = Record<string, string>

/**
 * Per-call options beyond headers. `signal` lets a caller (most often a
 * TanStack Query `queryFn`, which is handed an AbortSignal that fires on
 * unmount or when the query is superseded) cancel the in-flight request.
 */
export interface RequestOptions {
  signal?: AbortSignal
}

function defaultHeaders(): Record<string, string> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  const token =
    typeof window !== 'undefined' ? localStorage.getItem('alexandryn_access_token') : null
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  const activeLib =
    typeof window !== 'undefined' ? localStorage.getItem('alexandryn_active_library') : null
  if (activeLib) {
    headers['X-Library-Id'] = activeLib
  }
  return headers
}

export async function getJson<T>(
  path: string,
  headers: ExtraHeaders = {},
  opts: RequestOptions = {},
): Promise<T> {
  const res = await fetch(apiUrl(path), {
    headers: { ...defaultHeaders(), ...headers },
    signal: opts.signal,
  })
  return handleResponse<T>(res)
}

/**
 * Fetches a resource as a Blob through the same auth/library headers as
 * every other call. Returns the blob and the server's
 * suggested filename, if any.
 */
export async function getBlob(
  path: string,
  headers: ExtraHeaders = {},
  opts: RequestOptions = {},
): Promise<{ blob: Blob; filename: string | undefined }> {
  const res = await fetch(apiUrl(path), {
    headers: { ...defaultHeaders(), ...headers },
    signal: opts.signal,
  })
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as Partial<ApiErrorBody> | null
    throw new ApiError(res.status, body)
  }
  const disposition = res.headers.get('Content-Disposition') ?? ''
  const match = /filename="?([^"]+)"?/.exec(disposition)
  return { blob: await res.blob(), filename: match?.[1] }
}

/** Fetches a resource as text — the reader's sanitised chapter content. */
export async function getText(path: string, opts: RequestOptions = {}): Promise<string> {
  const res = await fetch(apiUrl(path), { headers: defaultHeaders(), signal: opts.signal })
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as Partial<ApiErrorBody> | null
    throw new ApiError(res.status, body)
  }
  return res.text()
}

export async function postJson<T>(
  path: string,
  body?: unknown,
  headers: ExtraHeaders = {},
  opts: RequestOptions = {},
): Promise<T> {
  const res = await fetch(apiUrl(path), {
    method: 'POST',
    headers: {
      ...defaultHeaders(),
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal: opts.signal,
  })
  return handleResponse<T>(res)
}

export async function putJson<T>(
  path: string,
  body?: unknown,
  headers: ExtraHeaders = {},
  opts: RequestOptions = {},
): Promise<T> {
  const res = await fetch(apiUrl(path), {
    method: 'PUT',
    headers: {
      ...defaultHeaders(),
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal: opts.signal,
  })
  return handleResponse<T>(res)
}

export async function patchJson<T>(
  path: string,
  body?: unknown,
  headers: ExtraHeaders = {},
  opts: RequestOptions = {},
): Promise<T> {
  const res = await fetch(apiUrl(path), {
    method: 'PATCH',
    headers: {
      ...defaultHeaders(),
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal: opts.signal,
  })
  return handleResponse<T>(res)
}

export async function deleteRequest(
  path: string,
  headers: ExtraHeaders = {},
  opts: RequestOptions = {},
): Promise<void> {
  const res = await fetch(apiUrl(path), {
    method: 'DELETE',
    headers: { ...defaultHeaders(), ...headers },
    signal: opts.signal,
  })
  return handleResponse<void>(res)
}
