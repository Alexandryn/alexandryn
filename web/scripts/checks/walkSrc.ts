import { walkFilesByExtension } from './walkFiles.ts'

const SOURCE_EXTENSIONS = new Set(['.ts', '.tsx'])

/** Recursively lists every .ts/.tsx file under dir. */
export function walkSourceFiles(dir: string): string[] {
  return walkFilesByExtension(dir, SOURCE_EXTENSIONS)
}
