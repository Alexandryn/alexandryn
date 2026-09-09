import { readFileSync } from 'node:fs'
import { walkSourceFiles } from './walkSrc.ts'

// CSS properties whose value is px-shaped when written as a bare number in a
// React inline style object (React appends "px" itself — no unit suffix
// appears in the source, so the px-suffix patterns below can't catch it).
// Deliberately excludes unitless-by-default properties (lineHeight, opacity,
// zIndex, flex, fontWeight, order) to avoid flagging those.
const PX_STYLE_PROPS = [
  'width',
  'height',
  'top',
  'left',
  'right',
  'bottom',
  'padding',
  'paddingTop',
  'paddingRight',
  'paddingBottom',
  'paddingLeft',
  'margin',
  'marginTop',
  'marginRight',
  'marginBottom',
  'marginLeft',
  'fontSize',
  'borderRadius',
  'borderWidth',
  'gap',
  'rowGap',
  'columnGap',
]

// Interim, grep-based (frontend-component-primitives.md FR-4/Acceptance
// criteria) until a lint rule exists. Catches the shapes a raw value takes
// in this codebase: a literal hex color (3/4/6/8 digits, covering CSS4
// alpha-hex like #1a1917ff), a literal pixel value inside a Tailwind
// arbitrary-value bracket or a quoted inline-style string, and a bare
// number assigned to a known px-shaped style property (React appends "px"
// itself, so the source text never contains the unit).
const RAW_VALUE_PATTERNS: RegExp[] = [
  /#[0-9a-fA-F]{3,8}\b/, // hex color, e.g. #fff, #1a1917, #1a1917ff
  /\[-?\d+(?:\.\d+)?px\]/, // Tailwind arbitrary bracket value, e.g. w-[16px]
  /:\s*['"`]-?\d+(?:\.\d+)?px['"`]/, // quoted inline style value, e.g. width: '16px'
  ...PX_STYLE_PROPS.map(
    // Requires an immediately preceding `{` or `,` (object-literal shape,
    // e.g. `{ width: 16 }` or `, width: 16`) rather than a bare word
    // boundary — a bare-boundary version also matched ordinary prose like
    // "or width:0 alone" in a test description, a false positive a real
    // review caught.
    (prop) => new RegExp(`[{,]\\s*${prop}\\s*:\\s*-?\\d+(?:\\.\\d+)?(?![\\d.%a-zA-Z])`),
  ), // bare-number inline style value, e.g. { width: 16 }
]

export interface TokenStylingFinding {
  file: string
  pattern: string
}

/**
 * Removes `//` line comments and block comments so prose never trips the
 * patterns — an issue reference like `#150` reads as a 3-digit shorthand
 * hex, and an aside like `rootMargin: '300px'` reads as an inline style.
 * The `[^:]` guard keeps `https://…` inside a string intact.
 */
function stripComments(src: string): string {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/.*$/gm, '$1')
}

/** Scans every .ts/.tsx file under componentsDir for a raw hex/px value outside the token set. */
export function findRawStyleValues(componentsDir: string): TokenStylingFinding[] {
  const findings: TokenStylingFinding[] = []
  for (const file of walkSourceFiles(componentsDir)) {
    // Test files carry issue references like `#150` in describe/it titles
    // and define no shipped styling — they are not held to FR-4.
    if (/\.test\.tsx?$/.test(file)) continue
    const contents = stripComments(readFileSync(file, 'utf8'))
    for (const pattern of RAW_VALUE_PATTERNS) {
      if (pattern.test(contents)) {
        findings.push({ file, pattern: pattern.source })
      }
    }
  }
  return findings
}
