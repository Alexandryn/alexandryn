import { walkFilesByExtension } from './walkFiles.ts'

// Text-shaped file types worth grepping for secrets/MSW references.
// Deliberately excludes images/fonts/etc — nothing to match there, and
// scanning binary content would waste time without catching anything.
const SCANNABLE_EXTENSIONS = new Set(['.js', '.mjs', '.html', '.css', '.json', '.map', '.txt'])

/**
 * Recursively lists every scannable (text-shaped) file under distDir —
 * the whole tree, not just distDir/assets. Vite copies web/public/*
 * straight to dist's own root (favicons, and critically, MSW's own
 * generated mockServiceWorker.js when `npx msw init public/` is used),
 * so a check scoped to assets/ alone misses exactly the file it exists
 * to catch.
 */
export function walkScannableFiles(distDir: string): string[] {
  return walkFilesByExtension(distDir, SCANNABLE_EXTENSIONS)
}
