export interface MergedProperty {
  value: string
  canvases: string[]
}

/**
 * Unions each canvas's root custom properties into one map, recording
 * which canvas(es) declared each — not every property is universal
 * (e.g. `--cov` only exists in the Electron canvas, `--dz` only in
 * Mobile). Throws loudly if two canvases declare the same property
 * name with different values — a real design-reference inconsistency
 * to surface, never silently picked between.
 */
export function mergeCanvasProperties(
  canvases: Record<string, Record<string, string>>,
): Record<string, MergedProperty> {
  const merged: Record<string, MergedProperty> = {}
  for (const [canvasName, props] of Object.entries(canvases)) {
    for (const [key, value] of Object.entries(props)) {
      const existing = merged[key]
      if (!existing) {
        merged[key] = { value, canvases: [canvasName] }
      } else if (existing.value !== value) {
        throw new Error(
          `mergeCanvasProperties: ${key} values disagree — ${existing.canvases.join(',')} say ${existing.value}, ${canvasName} says ${value}`,
        )
      } else {
        existing.canvases.push(canvasName)
      }
    }
  }
  return merged
}
