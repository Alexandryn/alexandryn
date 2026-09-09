// Cover bytes on an import candidate are extracted from a book file a
// user's source provided — untrusted. When they arrive as a data: URI it
// is dropped straight into an <img src>, so only a fixed set of raster
// image types is allowed; anything else (image/svg+xml, text/html, …) is
// rejected and the caller falls back to a generated cover (audit 0016
// #171).

const ALLOWED_IMAGE_DATA_URI =
  /^data:image\/(jpeg|jpg|png|webp)(?:;[a-z0-9-]+=[^,;]*)*(?:;base64)?,/i

export function isAllowedImageDataUri(value: string): boolean {
  return ALLOWED_IMAGE_DATA_URI.test(value.trim())
}

/**
 * Returns a safe <img src> for candidate cover bytes: an allowed image
 * data: URI as-is, raw base64 wrapped as JPEG, or null when the value is
 * a data: URI of a disallowed type.
 */
export function coverImageSrc(coverBytes: string | undefined | null): string | null {
  if (!coverBytes) return null
  if (coverBytes.startsWith('data:')) {
    return isAllowedImageDataUri(coverBytes) ? coverBytes : null
  }
  return `data:image/jpeg;base64,${coverBytes}`
}
