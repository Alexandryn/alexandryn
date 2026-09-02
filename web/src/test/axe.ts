import axe from 'axe-core'

// jsdom has no layout engine, so axe-core's color-contrast rule can't compute
// real rendered contrast and produces unreliable results — disabled here, the
// same call every jsdom-based axe integration (e.g. jest-axe) makes. Real
// contrast is covered by tokens.check-contrast (Tier 1) and Tier 5's
// real-browser @axe-core/playwright stage (F24), not duplicated here.
const RULES: axe.RunOptions['rules'] = {
  'color-contrast': { enabled: false },
}

export async function runAxe(
  container: Element,
  options: { exclude?: string } = {},
): Promise<axe.Result[]> {
  // A sandboxed <iframe> can't be traversed under jsdom (no real frame
  // window); a caller testing a screen that embeds one excludes it here
  // and relies on the real-browser @axe-core/playwright stage for the
  // frame's own content.
  const excluded = options.exclude
    ? Array.from(container.querySelectorAll(options.exclude))
    : []
  const context: axe.ElementContext = excluded.length
    ? ({ include: [container], exclude: excluded } as unknown as axe.ElementContext)
    : container
  const results = await axe.run(context, { rules: RULES })
  return results.violations
}

export function expectNoAxeViolations(violations: axe.Result[]): void {
  if (violations.length === 0) return
  const message = violations
    .map((v) => `${v.id} (${v.impact}): ${v.help}\n${v.nodes.map((n) => n.html).join('\n')}`)
    .join('\n\n')
  throw new Error(`axe-core found ${violations.length} violation(s):\n\n${message}`)
}
