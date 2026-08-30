// desktop-host-window-and-serving.md FR-3 — boot asset client controller.
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
    retryBtn.addEventListener('click', () => {
      // Reload to retry startup
      window.location.search = ''
      window.location.hash = ''
      window.location.reload()
    })
  }
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initBootAsset)
} else {
  initBootAsset()
}
