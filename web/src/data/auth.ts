import { getJson, postJson } from './http'

export interface UserSummary {
  id: string
  username: string
  email: string
  role: 'admin' | 'reader'
}

export interface AuthResponse {
  user?: UserSummary
  accessToken?: string
  refreshToken?: string
  mfaRequired?: boolean
  mfaTicket?: string
}

export interface SetupStatusResponse {
  isSetup: boolean
}

export interface TOTPSetupResponse {
  secret: string
  keyUri: string
  recoveryCodes: string[]
}

const ACCESS_TOKEN_KEY = 'alexandryn_access_token'
const REFRESH_TOKEN_KEY = 'alexandryn_refresh_token'
const USER_KEY = 'alexandryn_user'
const ACTIVE_LIB_KEY = 'alexandryn_active_library'

export const AUTH_CHANGE_EVENT = 'alexandryn_auth_change'

export function notifyAuthChange(): void {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event(AUTH_CHANGE_EVENT))
  }
}

export function getAccessToken(): string | null {
  return typeof window !== 'undefined' ? localStorage.getItem(ACCESS_TOKEN_KEY) : null
}

export function parseJwtLibraries(token: string): string[] {
  try {
    const parts = token.split('.')
    if (parts.length < 2) return []
    const base64Url = parts[1]
    if (!base64Url) return []
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/')
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split('')
        .map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join(''),
    )
    const payload = JSON.parse(jsonPayload)
    return Array.isArray(payload.libraries) ? payload.libraries : []
  } catch {
    return []
  }
}

export function setAccessToken(token: string | null): void {
  if (typeof window !== 'undefined') {
    if (token) {
      localStorage.setItem(ACCESS_TOKEN_KEY, token)
      if (!getActiveLibraryId()) {
        const libs = parseJwtLibraries(token)
        if (libs.length > 0 && libs[0]) {
          localStorage.setItem(ACTIVE_LIB_KEY, libs[0])
        }
      }
    } else {
      localStorage.removeItem(ACCESS_TOKEN_KEY)
    }
    notifyAuthChange()
  }
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY)
}

export function setRefreshToken(token: string | null): void {
  if (token) {
    localStorage.setItem(REFRESH_TOKEN_KEY, token)
  } else {
    localStorage.removeItem(REFRESH_TOKEN_KEY)
  }
}

export function getCurrentUser(): UserSummary | null {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw)
  } catch {
    return null
  }
}

export function setCurrentUser(user: UserSummary | null): void {
  if (typeof window !== 'undefined') {
    if (user) {
      localStorage.setItem(USER_KEY, JSON.stringify(user))
    } else {
      localStorage.removeItem(USER_KEY)
    }
    notifyAuthChange()
  }
}

export function getActiveLibraryId(): string | null {
  return typeof window !== 'undefined' ? localStorage.getItem(ACTIVE_LIB_KEY) : null
}

export function setActiveLibraryId(id: string | null): void {
  if (typeof window !== 'undefined') {
    if (id) {
      localStorage.setItem(ACTIVE_LIB_KEY, id)
    } else {
      localStorage.removeItem(ACTIVE_LIB_KEY)
    }
    notifyAuthChange()
  }
}

export function clearSession(): void {
  setAccessToken(null)
  setRefreshToken(null)
  setCurrentUser(null)
  setActiveLibraryId(null)
  notifyAuthChange()
}

export function fetchSetupStatus(): Promise<SetupStatusResponse> {
  return getJson<SetupStatusResponse>('/api/v1/auth/setup/status')
}

export async function setupAdmin(data: { username: string; email: string; password: string }): Promise<AuthResponse> {
  const res = await postJson<AuthResponse>('/api/v1/auth/setup', data)
  if (res.accessToken) setAccessToken(res.accessToken)
  if (res.refreshToken) setRefreshToken(res.refreshToken)
  if (res.user) setCurrentUser(res.user)
  return res
}

export async function login(data: {
  emailOrUsername: string
  password: string
  enrolmentGrant?: string
}): Promise<AuthResponse> {
  const res = await postJson<AuthResponse>('/api/v1/auth/login', data)
  if (res.accessToken) setAccessToken(res.accessToken)
  if (res.refreshToken) setRefreshToken(res.refreshToken)
  if (res.user) setCurrentUser(res.user)
  return res
}

export async function refreshSession(): Promise<AuthResponse> {
  const rt = getRefreshToken()
  if (!rt) {
    clearSession()
    throw new Error('No refresh token available')
  }
  try {
    const res = await postJson<AuthResponse>('/api/v1/auth/refresh', { refreshToken: rt })
    if (res.accessToken) setAccessToken(res.accessToken)
    if (res.refreshToken) setRefreshToken(res.refreshToken)
    return res
  } catch (err) {
    clearSession()
    throw err
  }
}

export async function logout(): Promise<void> {
  const rt = getRefreshToken()
  try {
    if (rt) {
      await postJson('/api/v1/auth/logout', { refreshToken: rt })
    }
  } finally {
    clearSession()
  }
}

export function requestPasswordReset(data: { email: string }): Promise<{ message: string }> {
  return postJson('/api/v1/auth/password-reset/request', data)
}

export function confirmPasswordReset(data: { token: string; newPassword: string }): Promise<{ success: boolean }> {
  return postJson('/api/v1/auth/password-reset/confirm', data)
}

export function setupTOTP(): Promise<TOTPSetupResponse> {
  return postJson<TOTPSetupResponse>('/api/v1/auth/mfa/totp/setup')
}

export function confirmTOTP(code: string): Promise<{ enabled: boolean }> {
  return postJson('/api/v1/auth/mfa/totp/confirm', { code })
}

export async function verifyTOTP(data: {
  mfaTicket: string
  code?: string
  recoveryCode?: string
  enrolmentGrant?: string
}): Promise<AuthResponse> {
  const res = await postJson<AuthResponse>('/api/v1/auth/mfa/totp/verify', data)
  if (res.accessToken) setAccessToken(res.accessToken)
  if (res.refreshToken) setRefreshToken(res.refreshToken)
  if (res.user) setCurrentUser(res.user)
  return res
}

export function disableTOTP(password: string): Promise<{ disabled: boolean }> {
  return postJson('/api/v1/auth/mfa/totp/disable', { password })
}
