/**
 * Maps each design-reference custom-property name to a semantic Tailwind
 * token name (frontend-design-tokens.md FR-1: "semantic names — never
 * raw color names"). Fixed by inspection of what each variable is
 * actually used for across all four canvases, not guessed from the
 * variable name alone.
 */
export const COLOR_NAMES: Record<string, string> = {
  '--bg': 'background',
  '--sf': 'surface',
  '--sf2': 'surface-2',
  '--sf3': 'surface-3',
  '--tx': 'text',
  '--tx2': 'text-2',
  '--tx3': 'text-3',
  '--ln': 'border',
  '--ln2': 'border-2',
  '--ac': 'accent',
  '--acs': 'accent-soft',
  '--act': 'accent-text',
  '--wm': 'warm',
  '--ok': 'success',
  '--er': 'error',
  // Canvas-specific, not universal — present in one surface only. --dz
  // is a real color (Mobile's dark overlay backdrop); --cov is a size
  // (Electron's book-cover width), never a color, so it's not in this
  // map at all — see SIZE_NAMES.
  '--dz': 'scrim',
}

export const SHADOW_NAMES: Record<string, string> = {
  '--sh': 'shadow-sm',
  '--sh2': 'shadow-lg',
}

/** Non-color, non-shadow custom properties — a size, not a scale. */
export const SIZE_NAMES: Record<string, string> = {
  '--cov': 'cover-width', // Electron only: book-cover sizing
}
