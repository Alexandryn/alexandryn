import { describe, expect, it, vi } from 'vitest'
import {
  isSafeExternalUrl,
  openExternalIfSafe,
  setupWindowNavigation,
} from './navigation'

// External link interception.
// setWindowOpenHandler deny + scheme-validated shell.openExternal (http/https only).
// will-navigate interception for non-loopback/LAN destinations.

describe('isSafeExternalUrl (scheme validator)', () => {
  it('accepts standard http and https URLs', () => {
    expect(isSafeExternalUrl('http://example.com')).toBe(true)
    expect(isSafeExternalUrl('https://example.com/path?q=1')).toBe(true)
    expect(isSafeExternalUrl('https://openlibrary.org/works/OL123W')).toBe(true)
  })

  it('rejects dangerous and non-http schemes', () => {
    expect(isSafeExternalUrl('file:///etc/passwd')).toBe(false)
    expect(isSafeExternalUrl('javascript:alert(1)')).toBe(false)
    expect(isSafeExternalUrl('data:text/html,<b>evil</b>')).toBe(false)
    expect(isSafeExternalUrl('vbscript:msgbox(1)')).toBe(false)
    expect(isSafeExternalUrl('about:blank')).toBe(false)
    expect(isSafeExternalUrl('custom-protocol://test')).toBe(false)
  })

  it('rejects malformed or empty URLs', () => {
    expect(isSafeExternalUrl('')).toBe(false)
    expect(isSafeExternalUrl('not a url')).toBe(false)
  })
})

describe('openExternalIfSafe (opener)', () => {
  it('calls opener when URL is safe', () => {
    const opener = vi.fn().mockResolvedValue(undefined)
    const result = openExternalIfSafe('https://openlibrary.org', opener)
    expect(result).toBe(true)
    expect(opener).toHaveBeenCalledWith('https://openlibrary.org')
  })

  it('does NOT call opener and returns false when URL scheme is hostile/invalid', () => {
    const opener = vi.fn().mockResolvedValue(undefined)
    const result = openExternalIfSafe('file:///root/secret', opener)
    expect(result).toBe(false)
    expect(opener).not.toHaveBeenCalled()
  })
})

describe('setupWindowNavigation (webContents wiring)', () => {
  it('setWindowOpenHandler always returns action: deny and opens safe external URLs', () => {
    let windowOpenHandler!: (details: { url: string }) => { action: 'deny' }
    const opener = vi.fn().mockResolvedValue(undefined)

    const mockWebContents = {
      setWindowOpenHandler: (handler: (details: { url: string }) => { action: 'deny' }) => {
        windowOpenHandler = handler
      },
      on: vi.fn(),
    } as unknown as import('electron').WebContents

    setupWindowNavigation(mockWebContents, { opener })

    expect(windowOpenHandler).toBeDefined()

    // Legitimate link: action is deny, opener is invoked
    const res1 = windowOpenHandler({ url: 'https://example.com' })
    expect(res1).toEqual({ action: 'deny' })
    expect(opener).toHaveBeenCalledWith('https://example.com')

    // Hostile link: action is deny, opener is NOT invoked
    opener.mockClear()
    const res2 = windowOpenHandler({ url: 'file:///etc/hosts' })
    expect(res2).toEqual({ action: 'deny' })
    expect(opener).not.toHaveBeenCalled()
  })

  it('will-navigate intercepts non-allowed origins and prevents default navigation', () => {
    const listeners: Record<string, (event: { preventDefault: () => void }, url: string) => void> = {}
    const opener = vi.fn().mockResolvedValue(undefined)

    const mockWebContents = {
      setWindowOpenHandler: vi.fn(),
      on: (event: string, fn: (event: { preventDefault: () => void }, url: string) => void) => {
        listeners[event] = fn
      },
    } as unknown as import('electron').WebContents

    const allowedOrigin = 'http://127.0.0.1:45123'
    setupWindowNavigation(mockWebContents, { allowedOrigin, opener })

    const willNavigate = listeners['will-navigate']
    expect(willNavigate).toBeDefined()

    // Navigation to allowed server origin is NOT prevented
    const allowEvent = { preventDefault: vi.fn() }
    willNavigate!(allowEvent, 'http://127.0.0.1:45123/books')
    expect(allowEvent.preventDefault).not.toHaveBeenCalled()
    expect(opener).not.toHaveBeenCalled()

    // Navigation to external origin is prevented and routed to opener
    const blockEvent = { preventDefault: vi.fn() }
    willNavigate!(blockEvent, 'https://external-site.com/page')
    expect(blockEvent.preventDefault).toHaveBeenCalled()
    expect(opener).toHaveBeenCalledWith('https://external-site.com/page')
  })

  it('will-navigate supports dynamic getAllowedOrigin getter', () => {
    const listeners: Record<string, (event: { preventDefault: () => void }, url: string) => void> = {}
    const opener = vi.fn().mockResolvedValue(undefined)

    const mockWebContents = {
      setWindowOpenHandler: vi.fn(),
      on: (event: string, fn: (event: { preventDefault: () => void }, url: string) => void) => {
        listeners[event] = fn
      },
    } as unknown as import('electron').WebContents

    let dynamicPort: number | undefined = undefined
    setupWindowNavigation(mockWebContents, {
      getAllowedOrigin: () => (dynamicPort !== undefined ? `http://127.0.0.1:${dynamicPort}` : undefined),
      opener,
    })

    const willNavigate = listeners['will-navigate']!

    // Before server ready (dynamicPort is undefined), navigation is prevented
    const event1 = { preventDefault: vi.fn() }
    willNavigate(event1, 'http://127.0.0.1:50000/ready')
    expect(event1.preventDefault).toHaveBeenCalled()

    // After server ready, dynamicPort matches
    dynamicPort = 50000
    const event2 = { preventDefault: vi.fn() }
    willNavigate(event2, 'http://127.0.0.1:50000/books')
    expect(event2.preventDefault).not.toHaveBeenCalled()
  })
})

