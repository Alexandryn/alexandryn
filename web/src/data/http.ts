// The one place the app calls fetch (frontend-shell-and-routing.md FR-2):
// every server-derived value goes through a TanStack Query hook whose
// queryFn calls one of these helpers, never fetch inline in a component.

export interface ApiErrorBody {
  code: string
  message: string
  correlationId: string
}

/** A non-2xx API response, carrying architecture-contracts.md FR-5's error shape. */
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
  if (res.status === 204) {
    return undefined as unknown as T
  }
  return (await res.json()) as T
}

/** Extra request headers — used by the reader for `X-Device-Id`. */
export type ExtraHeaders = Record<string, string>

export async function getJson<T>(path: string, headers: ExtraHeaders = {}): Promise<T> {
  const res = await fetch(apiUrl(path), { headers: { Accept: 'application/json', ...headers } })
  return handleResponse<T>(res)
}

/** Fetches a resource as text — the reader's sanitised chapter content. */
export async function getText(path: string): Promise<string> {
  const res = await fetch(apiUrl(path))
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
): Promise<T> {
  const res = await fetch(apiUrl(path), {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  return handleResponse<T>(res)
}

export async function putJson<T>(
  path: string,
  body?: unknown,
  headers: ExtraHeaders = {},
): Promise<T> {
  const res = await fetch(apiUrl(path), {
    method: 'PUT',
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  return handleResponse<T>(res)
}

export async function patchJson<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(apiUrl(path), {
    method: 'PATCH',
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  return handleResponse<T>(res)
}

export async function deleteRequest(path: string): Promise<void> {
  const res = await fetch(apiUrl(path), {
    method: 'DELETE',
    headers: { Accept: 'application/json' },
  })
  return handleResponse<void>(res)
}
