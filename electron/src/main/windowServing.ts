import type { BrowserWindow } from 'electron'
import { MAX_RESPAWN_ATTEMPTS, type ServerEvent } from './serverLifecycle'

// LoadURL sequencing & banners.
// Not installed → loadFile(bootHtmlPath, { query: { state: 'error', message }, hash: '#error' })
// Initializing  → loadFile(bootHtmlPath)
// Starting      → loadFile(bootHtmlPath)
// Healthy/Ready → loadURL('http://127.0.0.1:<port>')
// Degraded      → inject degraded banner over existing real UI
// Recovering    → inject banner over existing real UI via insertCSS + executeJavaScript
// Healthy (after Recovering/Degraded) → remove banners (and fresh loadURL if port changed)
// Failed        → loadFile(bootHtmlPath, { query: { state: 'error', message }, hash: '#error' })
// Stopped       → clean banners

export const RECOVERING_BANNER_CSS = `
#alexandryn-recovering-banner {
  position: fixed;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 999999;
  background: #ffffff;
  border: 1px solid #e7e4de;
  border-radius: 8px;
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  box-shadow: 0 4px 12px rgba(24, 22, 20, 0.15);
  font-family: system-ui, -apple-system, sans-serif;
  font-size: 13px;
  color: #1a1917;
  user-select: none;
}
#alexandryn-recovering-banner .banner-spinner {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  border: 2px solid #e7e4de;
  border-top-color: #41608f;
  animation: alexandryn-spin 0.9s linear infinite;
}
@keyframes alexandryn-spin {
  to { transform: rotate(360deg); }
}
`

export const DEGRADED_BANNER_CSS = `
#alexandryn-degraded-banner {
  position: fixed;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 999999;
  background: #fff8e6;
  border: 1px solid #d4a72c;
  border-radius: 8px;
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  box-shadow: 0 4px 12px rgba(24, 22, 20, 0.15);
  font-family: system-ui, -apple-system, sans-serif;
  font-size: 13px;
  color: #73510d;
  user-select: none;
}
#alexandryn-degraded-banner .banner-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: #d4a72c;
}
`

export class WindowServingController {
  private readonly window: BrowserWindow
  private readonly bootHtmlPath: string
  private currentPort?: number
  private recoveringCssKey?: string
  private degradedCssKey?: string
  private realUiLoaded = false

  constructor(window: BrowserWindow, bootHtmlPath: string) {
    this.window = window
    this.bootHtmlPath = bootHtmlPath
  }

  getCurrentPort(): number | undefined {
    return this.currentPort
  }

  isRealUiLoaded(): boolean {
    return this.realUiLoaded
  }

  async handleServerEvent(event: ServerEvent): Promise<void> {
    if (typeof this.window.isDestroyed === 'function' && this.window.isDestroyed()) {
      return
    }

    switch (event.state) {
      case 'Not installed': {
        await this.removeRecoveringBanner()
        await this.removeDegradedBanner()
        this.realUiLoaded = false
        const currentUrl =
          typeof this.window.webContents?.getURL === 'function'
            ? this.window.webContents.getURL()
            : undefined
        await this.window.loadFile(this.bootHtmlPath, {
          query: {
            state: 'error',
            message: event.message || 'Server binary is not installed.',
          },
          hash: '#error',
        })
        if (
          currentUrl &&
          typeof this.window.webContents?.getURL === 'function' &&
          this.window.webContents.getURL() === currentUrl &&
          typeof this.window.webContents?.reload === 'function'
        ) {
          this.window.webContents.reload()
        }
        break
      }

      case 'Initializing':
      case 'Starting': {
        if (!this.realUiLoaded) {
          await this.window.loadFile(this.bootHtmlPath)
        }
        break
      }

      case 'Healthy':
      case 'Ready': {
        const newPort = event.port
        if (newPort === undefined) return

        await this.removeDegradedBanner()

        if (this.recoveringCssKey !== undefined) {
          // Returning from Recovering state
          if (newPort !== this.currentPort) {
            this.currentPort = newPort
            await this.window.loadURL(`http://127.0.0.1:${newPort}`)
          }
          await this.removeRecoveringBanner()
        } else if (!this.realUiLoaded || newPort !== this.currentPort) {
          // First ready transition or port changed
          this.currentPort = newPort
          this.realUiLoaded = true
          await this.window.loadURL(`http://127.0.0.1:${newPort}`)
        }
        break
      }

      case 'Degraded': {
        if (this.realUiLoaded) {
          await this.injectDegradedBanner(event.message || 'Database connection unavailable')
        }
        break
      }

      case 'Recovering': {
        await this.removeDegradedBanner()
        if (this.realUiLoaded) {
          await this.injectRecoveringBanner(event.attempt ?? 1)
        }
        break
      }

      case 'Failed': {
        await this.removeRecoveringBanner()
        await this.removeDegradedBanner()
        this.realUiLoaded = false
        const currentUrl =
          typeof this.window.webContents?.getURL === 'function'
            ? this.window.webContents.getURL()
            : undefined
        await this.window.loadFile(this.bootHtmlPath, {
          query: {
            state: 'error',
            ...(event.message ? { message: event.message } : {}),
          },
          hash: '#error',
        })
        if (
          currentUrl &&
          typeof this.window.webContents?.getURL === 'function' &&
          this.window.webContents.getURL() === currentUrl &&
          typeof this.window.webContents?.reload === 'function'
        ) {
          this.window.webContents.reload()
        }
        break
      }

      case 'Stopped': {
        await this.removeRecoveringBanner()
        await this.removeDegradedBanner()
        break
      }
    }
  }

  private async injectDegradedBanner(message: string): Promise<void> {
    try {
      if (this.degradedCssKey === undefined) {
        this.degradedCssKey = await this.window.webContents.insertCSS(DEGRADED_BANNER_CSS)
      }

      const script = `
(function(msg) {
  let el = document.getElementById('alexandryn-degraded-banner');
  if (!el) {
    el = document.createElement('div');
    el.id = 'alexandryn-degraded-banner';
    el.setAttribute('role', 'alert');
    el.setAttribute('aria-live', 'assertive');
    document.body.appendChild(el);
  }
  const dot = document.createElement('div');
  dot.className = 'banner-dot';
  const text = document.createElement('span');
  text.textContent = 'Service Degraded: ' + msg;
  el.replaceChildren(dot, text);
})(${JSON.stringify(message)})
`
      await this.window.webContents.executeJavaScript(script)
    } catch {
      // Non-fatal
    }
  }

  private async removeDegradedBanner(): Promise<void> {
    try {
      if (this.degradedCssKey !== undefined) {
        await this.window.webContents.removeInsertedCSS(this.degradedCssKey)
        this.degradedCssKey = undefined
      }

      const script = `
(function() {
  const el = document.getElementById('alexandryn-degraded-banner');
  if (el) el.remove();
})()
`
      await this.window.webContents.executeJavaScript(script)
    } catch {
      // Non-fatal
    }
  }

  private async injectRecoveringBanner(attempt: number): Promise<void> {
    try {
      if (this.recoveringCssKey === undefined) {
        this.recoveringCssKey = await this.window.webContents.insertCSS(RECOVERING_BANNER_CSS)
      }

      const attemptNum = Math.max(1, Math.floor(Number(attempt) || 1))
      const maxNum = MAX_RESPAWN_ATTEMPTS
      const script = `
(function(attempt, max) {
  let el = document.getElementById('alexandryn-recovering-banner');
  if (!el) {
    el = document.createElement('div');
    el.id = 'alexandryn-recovering-banner';
    el.setAttribute('role', 'status');
    el.setAttribute('aria-live', 'polite');
    document.body.appendChild(el);
  }
  const spinner = document.createElement('div');
  spinner.className = 'banner-spinner';
  const text = document.createElement('span');
  text.textContent = 'Reconnecting to server... (Attempt ' + attempt + ' of ' + max + ')';
  el.replaceChildren(spinner, text);
})(${JSON.stringify(attemptNum)}, ${JSON.stringify(maxNum)})
`
      await this.window.webContents.executeJavaScript(script)
    } catch {
      // Non-fatal if webContents is not ready
    }
  }

  private async removeRecoveringBanner(): Promise<void> {
    try {
      if (this.recoveringCssKey !== undefined) {
        await this.window.webContents.removeInsertedCSS(this.recoveringCssKey)
        this.recoveringCssKey = undefined
      }

      const script = `
(function() {
  const el = document.getElementById('alexandryn-recovering-banner');
  if (el) el.remove();
})()
`
      await this.window.webContents.executeJavaScript(script)
    } catch {
      // Non-fatal cleanup
    }
  }
}
