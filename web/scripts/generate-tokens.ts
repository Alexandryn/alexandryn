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
 * FR-2: both this file and its sibling are written from the same single
 * extraction pass, so they can never drift from each other. */
`

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
    "  --font-ui: Geist, system-ui, -apple-system, 'Helvetica Neue', sans-serif;",
    '  --font-reading: Newsreader, Georgia, serif;',
    "  --font-mono: 'IBM Plex Mono', monospace;",
  )
  lines.push('}', '')
  return lines.join('\n')
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
const prettierConfig = (await prettier.resolveConfig(themePath)) ?? {}

writeFileSync(
  themePath,
  await prettier.format(buildThemeCss(tokens), { ...prettierConfig, filepath: themePath }),
)
writeFileSync(
  tokensPath,
  await prettier.format(buildTokensCss(tokens), { ...prettierConfig, filepath: tokensPath }),
)

console.log(
  `tokens:generate — ${tokens.colors.length} colors, ${tokens.shadows.length} shadows, ` +
    `${tokens.sizes.length} sizes, ${tokens.radius.length} radius, ${tokens.spacing.length} spacing, ` +
    `${tokens.fontSize.length} font sizes, ${tokens.letterSpacing.length} letter-spacing values, ` +
    `breakpoint ${tokens.breakpoint.name}=${tokens.breakpoint.px}px ` +
    `→ src/theme.css, src/tokens.css`,
)
