// Boot asset client controller.
// Switches between loading and error states based on query parameter or hash.

function initBootAsset(): void {
  const params = new URLSearchParams(window.location.search)
  const isError = params.get('state') === 'error' || window.location.hash === '#error'

  const loadingView = document.getElementById('loading-state')
  const errorView = document.getElementById('error-state')
  const retryBtn = document.getElementById('retry-btn')

  if (isError) {
    if (loadingView) loadingView.style.display = 'none'
    if (errorView) errorView.style.display = 'block'
  } else {
    if (loadingView) loadingView.style.display = 'block'
    if (errorView) errorView.style.display = 'none'
  }

  if (retryBtn) {
    retryBtn.addEventListener('click', async () => {
      // Optimistically show loading view
      if (loadingView) loadingView.style.display = 'block'
      if (errorView) errorView.style.display = 'none'

      const alex = (window as unknown as { alexandryn?: { system?: { retryStartup?: () => Promise<void> } } }).alexandryn
      if (alex?.system?.retryStartup) {
        try {
          await alex.system.retryStartup()
        } catch {
          // If IPC call rejected, fallback to reload
          window.location.reload()
        }
      } else {
        window.location.reload()
      }
    })
  }
}







if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initBootAsset)
} else {
  initBootAsset()
}
