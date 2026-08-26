import { existsSync } from 'node:fs'

/**
 * Guards a CLI check against a missing directory with a clear, actionable
 * message instead of a raw ENOENT stack trace from deep inside the
 * checker (a real gap code review found: the three check-*.ts wrappers
 * only ever guarded distDir's own existence, not the subdirectory their
 * checker actually reads).
 */
export function requireDir(dir: string, checkName: string): void {
  if (!existsSync(dir)) {
    console.error(`${checkName}: ${dir} does not exist — run "npm run build" first`)
    process.exit(1)
  }
}

interface Finding {
  file: string
  pattern: string
}

/** Reports findings (if any) and exits — shared by the two grep-style checks. */
export function reportFindingsAndExit(
  checkName: string,
  findings: Finding[],
  description: string,
): never {
  if (findings.length > 0) {
    console.error(`${checkName}: ${description}`)
    for (const f of findings) {
      console.error(`  ${f.file}: matched ${f.pattern}`)
    }
    process.exit(1)
  }
  console.log(`${checkName}: clean`)
  process.exit(0)
}
