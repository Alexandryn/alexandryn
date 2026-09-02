// Minimal ambient types for the vendored foliate-js `epubcfi.js` — the
// standalone CFI module (ADR 0023). Only the functions the reader uses.

export const isCFI: RegExp

/** Build a CFI string from a DOM Range within a rendered document. */
export function fromRange(range: Range): string

/** Parse a CFI string. */
export function parse(cfi: string): unknown

/** Compare two CFI strings: negative if a precedes b, 0 if equal. */
export function compare(a: string, b: string): number

/** Join a spine-indirection CFI with a local CFI (`!` separator). */
export function joinIndir(...parts: string[]): string

/** Collapse a CFI to a single point (its start, by default). */
export function collapse(cfi: string, toEnd?: boolean): string
