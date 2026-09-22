// Type declaration for the plain build script fetch-postgres-binaries.mjs
// (not itself typechecked), so its one pure, unit-tested export has a real
// type at its one import site (fetchPostgresBinaries.test.ts) instead of
// silencing the whole module with @ts-expect-error.
export declare function readZipEntry(zip: Buffer, entryName: string): Buffer
