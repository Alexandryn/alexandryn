import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

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

/** Scans every .js file under distDir/assets for secret-shaped strings. */
export function findSecrets(distDir: string): SecretFinding[] {
  const assetsDir = join(distDir, 'assets')
  const findings: SecretFinding[] = []
  for (const name of readdirSync(assetsDir)) {
    if (!name.endsWith('.js')) continue
    const contents = readFileSync(join(assetsDir, name), 'utf8')
    for (const pattern of SECRET_PATTERNS) {
      if (pattern.test(contents)) {
        findings.push({ file: join(assetsDir, name), pattern: pattern.source })
      }
    }
  }
  return findings
}
