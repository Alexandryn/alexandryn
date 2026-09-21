// Cover bytes on an import candidate are extracted from a book file a
// user's source provided — untrusted. When they arrive as a data: URI it
// is dropped straight into an <img src>, so only a fixed set of raster
// image types is allowed; anything else (image/svg+xml, text/html, …) is
// rejected and the caller falls back to a generated cover.

const ALLOWED_IMAGE_DATA_URI =
  /^data:image\/(jpeg|jpg|png|webp)(?:;[a-z0-9-]+=[^,;]*)*(?:;base64)?,/i

// A cover thumbnail is tens of KB of encoded string; anything past ~1.5 MB
// is treated as hostile and dropped for the generated fallback
// (a size limit on untrusted input, not only a shape check).
const MAX_COVER_SRC_LENGTH = 1_500_000

export function isAllowedImageDataUri(value: string): boolean {
  return ALLOWED_IMAGE_DATA_URI.test(value.trim())
}

/**
 * Returns a safe <img src> for candidate cover bytes: an allowed image
 * data: URI as-is, raw base64 wrapped as JPEG, or null when the value is
 * empty, oversized, or a data: URI of a disallowed type.
 */
export function coverImageSrc(coverBytes: string | undefined | null): string | null {
  if (!coverBytes) return null
  const value = coverBytes.trim()
  if (!value || value.length > MAX_COVER_SRC_LENGTH) return null
  if (value.startsWith('data:')) {
    return isAllowedImageDataUri(value) ? value : null
  }
  return `data:image/jpeg;base64,${value}`
}
