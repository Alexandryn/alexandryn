// A device-pairing enrolment grant is handed from ConnectScreen to
// LoginScreen via router state, which a page reload discards — leaving
// the user unable to finish enrolling the device.
// Mirror it into sessionStorage (per-tab, cleared when the tab closes)
// so a reload on /login recovers it. It is a short-lived bearer secret,
// so it must NOT go in the URL or localStorage.

const KEY = 'alexandryn_pending_enrolment'

export interface PendingEnrolment {
  enrolmentGrant: string
  hostName?: string
}

export function setPendingEnrolment(value: PendingEnrolment): void {
  try {
    sessionStorage.setItem(KEY, JSON.stringify(value))
  } catch {
    // Private mode or storage disabled — the router-state path still
    // works for a user who does not reload.
  }
}

export function getPendingEnrolment(): PendingEnrolment | null {
  try {
    const raw = sessionStorage.getItem(KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<PendingEnrolment>
    return typeof parsed.enrolmentGrant === 'string'
      ? { enrolmentGrant: parsed.enrolmentGrant, hostName: parsed.hostName }
      : null
  } catch {
    return null
  }
}

export function clearPendingEnrolment(): void {
  try {
    sessionStorage.removeItem(KEY)
  } catch {
    // ignore
  }
}
