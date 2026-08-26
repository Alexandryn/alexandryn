import { readFileSync } from 'node:fs'
import { walkScannableFiles } from './walkDist.ts'

// Interim, grep-based — not a full entropy/secret-scanning tool. Catches
// the well-known shaped patterns; anything more thorough is a separate,
// deliberate future decision, not silently expanded here.
const SECRET_PATTERNS: RegExp[] = [
  /AKIA[0-9A-Z]{16}/, // AWS access key ID
  /(api[_-]?key|secret|token)["']?\s*[:=]\s*["'][A-Za-z0-9_-]{16,}["']/i, // generic key/secret/token assignment
]

export interface SecretFinding {
  file: string
  pattern: string
}

/** Scans every scannable file in distDir (the whole tree, not just assets/) for secret-shaped strings. */
export function findSecrets(distDir: string): SecretFinding[] {
  const findings: SecretFinding[] = []
  for (const file of walkScannableFiles(distDir)) {
    const contents = readFileSync(file, 'utf8')
    for (const pattern of SECRET_PATTERNS) {
      if (pattern.test(contents)) {
        findings.push({ file, pattern: pattern.source })
      }
    }
  }
  return findings
}
