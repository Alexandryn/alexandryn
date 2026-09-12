import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ErrorState } from '../components/ErrorState/ErrorState'
import { GeneratedCover } from '../components/GeneratedCover/GeneratedCover'
import { TitleLayer } from '../components/GeneratedCover/TitleLayer'
import { AuthorLayer } from '../components/GeneratedCover/AuthorLayer'

// Security guard: every component that renders source-, file-, or
// metadata-provider-derived text — titles, authors, and an API error's
// code / message / correlationId — must render it as an escaped text node,
// never as markup. React's default escaping gives us this for free as long as
// no component reaches for dangerouslySetInnerHTML or an innerHTML sink.
// These tests fail the moment one does.

const HTML_PAYLOAD = '<script>window.__xss = 1</script><img src=x onerror="window.__xss = 1">'

function assertNoInjectedMarkup(container: HTMLElement) {
  expect(container.querySelector('script')).toBeNull()
  // An <img> the component itself renders is fine; one carrying the
  // payload's onerror attribute is not.
  for (const img of container.querySelectorAll('img')) {
    expect(img.getAttribute('onerror')).toBeNull()
    expect(img.getAttribute('src')).not.toBe('x')
  }
  // The payload survives only as visible text.
  expect(container.textContent).toContain(HTML_PAYLOAD)
}

describe('XSS escaping — provider-derived text renders as text, never markup', () => {
  it('ErrorState escapes title, description, code and correlationId', () => {
    const { container } = render(
      <ErrorState
        title={HTML_PAYLOAD}
        description={HTML_PAYLOAD}
        code={HTML_PAYLOAD}
        correlationId={HTML_PAYLOAD}
      />,
    )
    assertNoInjectedMarkup(container)
  })

  it('TitleLayer escapes the title string', () => {
    const { container } = render(<TitleLayer title={HTML_PAYLOAD} />)
    assertNoInjectedMarkup(container)
  })

  it('AuthorLayer escapes the author string', () => {
    const { container } = render(<AuthorLayer author={HTML_PAYLOAD} />)
    assertNoInjectedMarkup(container)
  })

  it('GeneratedCover escapes title and author in the full composition', () => {
    const { container } = render(
      <GeneratedCover identifier="xss-work-1" title={HTML_PAYLOAD} author={HTML_PAYLOAD} />,
    )
    assertNoInjectedMarkup(container)
  })

  it('GeneratedCover derives its spine hue from a bounded number, not an interpolated string', () => {
    // seed.hue is fnv1a(identifier) % 360 — a caller cannot influence the
    // CSS beyond a 0–359 integer, so there is no `style` string-injection
    // surface even though the layers build `hsl(...)` by template literal.
    const { container } = render(<GeneratedCover identifier="}; background: url(//evil)" />)
    const withBg = Array.from(container.querySelectorAll<HTMLElement>('[style]')).map(
      (el) => el.style.backgroundColor,
    )
    for (const value of withBg) {
      if (!value) continue
      expect(value).toMatch(/^(rgb|hsl)\(/)
      expect(value).not.toContain('url(')
    }
  })
})
