import type { BrowserWindow } from 'electron'
import type { ServerEvent } from './serverLifecycle'

// desktop-host-window-and-serving.md FR-5, FR-6 — loadURL sequencing & Recovering banner.
// Starting   → loadFile(bootHtmlPath)
// Ready      → loadURL('http://127.0.0.1:<port>')
// Recovering → inject banner over existing real UI via insertCSS + executeJavaScript
// Ready (after Recovering) → remove banner (and fresh loadURL if port changed)
// Failed     → loadFile(bootHtmlPath, { query: { state: 'error' }, hash: '#error' })

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

export class WindowServingController {
  private readonly window: BrowserWindow
  private readonly bootHtmlPath: string
  private currentPort?: number
  private recoveringCssKey?: string
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
      case 'Starting': {
        this.realUiLoaded = false
        await this.window.loadFile(this.bootHtmlPath)
        break
      }

      case 'Ready': {
        const newPort = event.port
        if (newPort === undefined) return

        if (this.recoveringCssKey !== undefined) {
          // Returning from Recovering state
          if (newPort !== this.currentPort) {
            this.currentPort = newPort
            await this.window.loadURL(`http://127.0.0.1:${newPort}`)
          }
          await this.removeRecoveringBanner()
        } else {
          // First ready transition
          this.currentPort = newPort
          this.realUiLoaded = true
          await this.window.loadURL(`http://127.0.0.1:${newPort}`)
        }
        break
      }

      case 'Recovering': {

        if (this.realUiLoaded) {
          await this.injectRecoveringBanner(event.attempt ?? 1)
        }
        break
      }

      case 'Failed': {
        if (this.recoveringCssKey !== undefined) {
          await this.removeRecoveringBanner()
        }
        this.realUiLoaded = false
        await this.window.loadFile(this.bootHtmlPath, {
          query: { state: 'error' },
          hash: '#error',
        })
        break
      }

    }
  }


  private async injectRecoveringBanner(attempt: number): Promise<void> {
    try {
      if (this.recoveringCssKey === undefined) {
        this.recoveringCssKey = await this.window.webContents.insertCSS(RECOVERING_BANNER_CSS)
      }

      const script = `
(function() {
  let el = document.getElementById('alexandryn-recovering-banner');
  if (!el) {
    el = document.createElement('div');
    el.id = 'alexandryn-recovering-banner';
    el.setAttribute('role', 'status');
    el.setAttribute('aria-live', 'polite');
    document.body.appendChild(el);
  }
  el.innerHTML = '<div class="banner-spinner"></div><span>Reconnecting to server... (Attempt ${attempt} of 3)</span>';
})()
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
