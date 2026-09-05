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

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_TOKEN_KEY)
}

export function setAccessToken(token: string | null): void {
  if (token) {
    localStorage.setItem(ACCESS_TOKEN_KEY, token)
  } else {
    localStorage.removeItem(ACCESS_TOKEN_KEY)
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
  if (user) {
    localStorage.setItem(USER_KEY, JSON.stringify(user))
  } else {
    localStorage.removeItem(USER_KEY)
  }
}

export function getActiveLibraryId(): string | null {
  return localStorage.getItem(ACTIVE_LIB_KEY)
}

export function setActiveLibraryId(id: string | null): void {
  if (id) {
    localStorage.setItem(ACTIVE_LIB_KEY, id)
  } else {
    localStorage.removeItem(ACTIVE_LIB_KEY)
  }
}

export function clearSession(): void {
  setAccessToken(null)
  setRefreshToken(null)
  setCurrentUser(null)
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
