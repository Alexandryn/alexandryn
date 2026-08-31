import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import * as prettier from 'prettier'
import { parse } from 'yaml'

// frontend-shell-and-routing.md FR-6 tier (a): for any endpoint the
// contract already defines, the mock fixture is generated from
// api/openapi.yaml, never hand-written. This script is that mechanism.
// It reads each operation's inline `application/json` response example
// and writes them into one generated module the mock handlers and tests
// import. Run via `npm run mocks:gen-fixtures`; CI fails if the checked-in
// output no longer matches (same staleness discipline as tokens:generate).

const CONTRACT = fileURLToPath(new URL('../../api/openapi.yaml', import.meta.url))
const OUT = fileURLToPath(new URL('../src/mocks/fixtures/generated/index.ts', import.meta.url))

interface MediaType {
  example?: unknown
}
interface Operation {
  operationId?: string
  responses?: Record<string, { content?: Record<string, MediaType> }>
}
interface Contract {
  paths?: Record<string, Record<string, Operation>>
}

const HTTP_METHODS = ['get', 'put', 'post', 'delete', 'patch', 'head', 'options']

function build(): Record<string, Record<string, unknown>> {
  const contract = parse(readFileSync(CONTRACT, 'utf8')) as Contract
  const out: Record<string, Record<string, unknown>> = {}

  for (const [path, item] of Object.entries(contract.paths ?? {})) {
    for (const method of HTTP_METHODS) {
      const op = item[method]
      if (!op) continue
      const operationId = op.operationId
      if (!operationId) {
        throw new Error(`${method.toUpperCase()} ${path} has no operationId`)
      }
      for (const [status, response] of Object.entries(op.responses ?? {})) {
        // 204 / 304 and any other no-body response: no content key in the
        // spec → no fixture to generate for this status code. This is
        // correct — not a spec authoring error.
        if (!response.content) continue
        const json = response.content['application/json']
        if (!json || !('example' in json)) {
          throw new Error(
            `${operationId} ${status}: no application/json example in the contract — ` +
              `tier-(a) fixtures need an inline example to generate from`,
          )
        }
        out[operationId] ??= {}
        out[operationId][status] = json.example
      }
    }
  }
  return out
}

const fixtures = build()

const body = `// GENERATED FILE — do not hand-edit.
// Run \`npm run mocks:gen-fixtures\` (web/scripts/gen-fixtures.ts) to
// regenerate from api/openapi.yaml. frontend-shell-and-routing.md FR-6
// tier (a): fixtures for contract-covered endpoints are generated, never
// hand-written. Hand-written fixtures for endpoints the contract does not
// cover yet live in ../handwritten/ and carry a TODO(phase-06) marker.

export const generatedFixtures = ${JSON.stringify(fixtures, null, 2)} as const
`

const prettierConfig = (await prettier.resolveConfig(OUT)) ?? {}
writeFileSync(OUT, await prettier.format(body, { ...prettierConfig, filepath: OUT }))
console.log(
  `gen-fixtures: ${Object.keys(fixtures).length} operation(s) written to src/mocks/fixtures/generated/index.ts`,
)
