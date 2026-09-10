import { readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import * as prettier from 'prettier'
import { extractTokens, type ExtractedTokens } from './tokens/extract.ts'

const DESIGN_REFERENCE_DIR = join(import.meta.dirname, '..', '..', '.design-reference')
const CANVASES: Record<string, string> = {
  electron: 'Alexandryn-Electron.dc.html',
  admin: 'Alexandryn-Electron-Admin.dc.html',
  web: 'Alexandryn-Web.dc.html',
  mobile: 'Alexandryn-Mobile.dc.html',
}

const GENERATED_HEADER = `/* GENERATED FILE — do not hand-edit.
 * Run \`npm run tokens:generate\` (web/scripts/generate-tokens.ts) to
 * regenerate from .design-reference/*.dc.html — frontend-design-tokens.md
 * FR-2: theme.css, tokens.css and breakpoints.ts are all written from the
 * same single extraction pass, so they can never drift from each other. */
`

// The web-font integration contract (audit 0016 #223). The font-family
// tokens below currently resolve to their system fallbacks; this note
// records the rules for adding the real faces so it is visible at the
// definition site. Emitted by the generator so it survives regeneration.
const FONT_INTEGRATION_NOTE = `  /* Design fonts (audit 0016 #223): currently render via system fallback.
     When custom web fonts are added: self-host .woff2 under public/fonts/,
     use @font-face { font-display: swap } (optional for mono), subset with
     unicode-range, preload max 1-2 first-paint faces, and never load from
     external CDNs (fonts.googleapis.com). */`

function buildThemeCss(tokens: ExtractedTokens): string {
  const lines: string[] = [GENERATED_HEADER, '@import "tailwindcss";', '', '@theme {']
  for (const c of tokens.colors) lines.push(`  --color-${c.name}: ${c.value};`)
  for (const s of tokens.shadows)
    lines.push(`  --shadow-${s.name.replace(/^shadow-/, '')}: ${s.value};`)
  for (const s of tokens.sizes) lines.push(`  --size-${s.name}: ${s.value};`)
  for (const t of tokens.radius) lines.push(`  --radius-${t.name}: ${t.px}px;`)
  for (const t of tokens.spacing) lines.push(`  --spacing-${t.name}: ${t.px}px;`)
  for (const t of tokens.fontSize) lines.push(`  --text-${t.name}: ${t.px}px;`)
  for (const t of tokens.letterSpacing) lines.push(`  --tracking-${t.name}: ${t.em}em;`)
  // The single sidebar↔tab-bar reflow breakpoint (frontend-shell-and-
  // routing.md FR-3), read from the atTablet band prose (extractBreakpoint).
  // Tailwind v4 turns --breakpoint-reflow into the `reflow:` variant.
  lines.push(`  --breakpoint-${tokens.breakpoint.name}: ${tokens.breakpoint.px}px;`)
  lines.push(
    FONT_INTEGRATION_NOTE,
    "  --font-ui: Geist, system-ui, -apple-system, 'Helvetica Neue', sans-serif;",
    '  --font-reading: Newsreader, Georgia, serif;',
    "  --font-mono: 'IBM Plex Mono', monospace;",
  )
  lines.push('}', '')
  return lines.join('\n')
}

// A third generated artifact (same single pass, FR-2): the breakpoint as
// a plain number for the one consumer that can't read CSS — the
// useShellLayout hook's matchMedia query. Keeps the shell's reflow point
// tracing to the design reference with no magic number in JS.
function buildBreakpointsTs(tokens: ExtractedTokens): string {
  return `${GENERATED_HEADER}
export const BREAKPOINTS = {
  ${tokens.breakpoint.name}: ${tokens.breakpoint.px},
} as const
`
}

function buildTokensCss(tokens: ExtractedTokens): string {
  const lines: string[] = [GENERATED_HEADER, ':root {']
  for (const c of tokens.colors) lines.push(`  --${c.name}: ${c.value};`)
  for (const s of tokens.shadows) lines.push(`  --${s.name}: ${s.value};`)
  for (const s of tokens.sizes) lines.push(`  --${s.name}: ${s.value};`)
  for (const t of tokens.radius) lines.push(`  --radius-${t.name}: ${t.px}px;`)
  for (const t of tokens.spacing) lines.push(`  --spacing-${t.name}: ${t.px}px;`)
  for (const t of tokens.fontSize) lines.push(`  --text-${t.name}: ${t.px}px;`)
  for (const t of tokens.letterSpacing) lines.push(`  --tracking-${t.name}: ${t.em}em;`)
  lines.push(`  --breakpoint-${tokens.breakpoint.name}: ${tokens.breakpoint.px}px;`)
  lines.push(
    FONT_INTEGRATION_NOTE,
    "  --font-ui: Geist, system-ui, -apple-system, 'Helvetica Neue', sans-serif;",
    '  --font-reading: Newsreader, Georgia, serif;',
    "  --font-mono: 'IBM Plex Mono', monospace;",
  )
  lines.push('}', '')
  return lines.join('\n')
}

const canvasHtml: Record<string, string> = {}
for (const [name, file] of Object.entries(CANVASES)) {
  canvasHtml[name] = readFileSync(join(DESIGN_REFERENCE_DIR, file), 'utf8')
}

const tokens = extractTokens(canvasHtml)

const themePath = join(import.meta.dirname, '..', 'src', 'theme.css')
const tokensPath = join(import.meta.dirname, '..', 'src', 'tokens.css')
const breakpointsPath = join(import.meta.dirname, '..', 'src', 'breakpoints.ts')
const prettierConfig = (await prettier.resolveConfig(themePath)) ?? {}

writeFileSync(
  themePath,
  await prettier.format(buildThemeCss(tokens), { ...prettierConfig, filepath: themePath }),
)
writeFileSync(
  tokensPath,
  await prettier.format(buildTokensCss(tokens), { ...prettierConfig, filepath: tokensPath }),
)
writeFileSync(
  breakpointsPath,
  await prettier.format(buildBreakpointsTs(tokens), {
    ...prettierConfig,
    filepath: breakpointsPath,
  }),
)

console.log(
  `tokens:generate — ${tokens.colors.length} colors, ${tokens.shadows.length} shadows, ` +
    `${tokens.sizes.length} sizes, ${tokens.radius.length} radius, ${tokens.spacing.length} spacing, ` +
    `${tokens.fontSize.length} font sizes, ${tokens.letterSpacing.length} letter-spacing values, ` +
    `breakpoint ${tokens.breakpoint.name}=${tokens.breakpoint.px}px ` +
    `→ src/theme.css, src/tokens.css, src/breakpoints.ts`,
)
