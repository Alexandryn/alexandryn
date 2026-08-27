export interface BreakpointToken {
  name: string
  px: number
}

// The design reference has no @media rules to count — its canvases are
// fixed-width artboards. The one place a responsive width is stated is the
// atTablet screen's own prose: "768–1023px. The sidebar becomes a 60px
// icon rail, ...". frontend-shell-and-routing.md FR-3 needs exactly one
// sidebar↔tab-bar breakpoint; the low end of that band (768) is where the
// desktop sidebar begins, below it is the phone tab-bar layout. Maintainer
// decision F1 (tasks/todo-p04-tier4-shell-and-routing.md) — a reasoned
// extraction from the reference's one numeric statement, not an invented
// value (frontend-design-tokens.md FR-1/FR-4).
const TABLET_BAND = /(\d{3,4})[–-]\d{3,4}px\.\s+The sidebar becomes/

/** The single FR-3 reflow breakpoint, read from the atTablet band prose. */
export function extractBreakpoint(canvasHtmlByName: Record<string, string>): BreakpointToken {
  for (const html of Object.values(canvasHtmlByName)) {
    const match = TABLET_BAND.exec(html)
    if (match) return { name: 'reflow', px: Number(match[1]) }
  }
  throw new Error(
    'extractBreakpoint: the atTablet responsive-band prose ("NNN–NNNpx. The sidebar ' +
      'becomes ...") is not present in any canvas — the design reference changed; ' +
      're-confirm the FR-3 breakpoint value before regenerating tokens',
  )
}
