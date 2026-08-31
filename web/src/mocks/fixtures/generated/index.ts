// GENERATED FILE — do not hand-edit.
// Run `npm run mocks:gen-fixtures` (web/scripts/gen-fixtures.ts) to
// regenerate from api/openapi.yaml. frontend-shell-and-routing.md FR-6
// tier (a): fixtures for contract-covered endpoints are generated, never
// hand-written. Hand-written fixtures for endpoints the contract does not
// cover yet live in ../handwritten/ and carry a TODO(phase-06) marker.

export const generatedFixtures = {
  getHealthz: {
    '200': {},
  },
  getReadyz: {
    '200': {},
    '503': {
      code: 'unavailable',
      message: 'not yet started',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  listLibrary: {
    '200': {
      works: [
        {
          id: '01JXXXXXXXXXXXXXXXXXXXXXXX',
          title: 'Middlemarch',
          subtitle: 'A Study of Provincial Life',
          authors: ['George Eliot'],
          isOwned: true,
          collections: [
            {
              id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
              name: 'Classics',
              addedAt: '2026-01-20T08:00:00Z',
            },
          ],
          addedAt: '2026-01-15T10:00:00Z',
        },
      ],
      nextCursor: null,
    },
    '400': {
      code: 'invalid_input',
      message: 'sort: unrecognized value "relevance" — must be one of: added_at, title',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getWork: {
    '200': {
      id: '01JXXXXXXXXXXXXXXXXXXXXXXX',
      title: 'Middlemarch',
      subtitle: '',
      authors: ['George Eliot'],
      subjects: ['Fiction'],
      originalLanguage: 'en',
      ownedEditions: [
        {
          id: '01JXXXXXXXXXXXXXXXXXXXXXXY',
          language: 'en',
          isbn: '9780141439549',
          publisher: 'Penguin Classics',
          publicationYear: 2003,
          addedAt: '2026-01-15T10:00:00Z',
          formats: ['epub'],
        },
      ],
      collections: [
        {
          id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
          name: 'Classics',
          addedAt: '2026-01-20T08:00:00Z',
        },
      ],
    },
    '400': {
      code: 'invalid_input',
      message: 'id: not a valid work ID',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'no work with that id',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  listCollections: {
    '200': {
      collections: [
        {
          id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
          name: 'Classics',
          workCount: 1,
        },
      ],
    },
  },
  createCollection: {
    '201': {
      id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
      name: 'Classics',
      works: [],
    },
    '400': {
      code: 'invalid_input',
      message: 'name: exceeds maximum length of 100 characters',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getCollection: {
    '200': {
      id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
      name: 'Classics',
      works: [
        {
          id: '01JXXXXXXXXXXXXXXXXXXXXXXX',
          title: 'Middlemarch',
          subtitle: 'A Study of Provincial Life',
          authors: ['George Eliot'],
          isOwned: true,
          collections: [
            {
              id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
              name: 'Classics',
              addedAt: '2026-01-20T08:00:00Z',
            },
          ],
          addedAt: '2026-01-15T10:00:00Z',
        },
      ],
    },
    '400': {
      code: 'invalid_input',
      message: 'id: not a valid collection ID',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'no collection with that id',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  deleteCollection: {
    '400': {
      code: 'invalid_input',
      message: 'id: not a valid collection ID',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'no collection with that id',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  renameCollection: {
    '200': {
      id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
      name: 'Renamed Collection',
      works: [],
    },
    '400': {
      code: 'invalid_input',
      message: 'name: contains control characters',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'no collection with that id',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  addWorkToCollection: {
    '200': {
      id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
      name: 'Classics',
      works: [],
    },
    '400': {
      code: 'invalid_input',
      message: 'workId: missing required field',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'no collection with that id',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  removeWorkFromCollection: {
    '400': {
      code: 'invalid_input',
      message: 'workId: not a valid work ID',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'no membership found for that work in this collection',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  searchDiscover: {
    '200': {
      items: [
        {
          openLibraryWorkKey: 'OL82563W',
          title: 'Middlemarch',
          authors: [
            {
              openLibraryAuthorKey: 'OL21594A',
              name: 'George Eliot',
            },
          ],
          firstPublishYear: 1871,
          coverUrl: '/api/v1/discover/covers/8256301',
          editionCount: 42,
        },
      ],
      total: 1,
      limit: 20,
      offset: 0,
    },
    '400': {
      code: 'invalid_input',
      message: 'q: required parameter missing',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '503': {
      code: 'unavailable',
      message: 'Open Library is temporarily unavailable',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getDiscoverWork: {
    '200': {
      work: {
        title: 'Middlemarch',
        subtitle: 'A Study of Provincial Life',
        description:
          'Middlemarch, A Study of Provincial Life is a novel by Mary Anne Evans, writing as George Eliot.',
        subjects: ['Provincial life', 'Fiction'],
        authors: [
          {
            openLibraryAuthorKey: 'OL21594A',
            name: 'George Eliot',
          },
        ],
        coverUrl: '/api/v1/discover/covers/8256301',
      },
      editions: [
        {
          title: 'Middlemarch',
          publisher: 'Penguin Classics',
          publishDate: '2003',
          language: 'en',
          openLibraryEditionKey: 'OL7353617M',
          coverUrl: '/api/v1/discover/covers/8256301',
        },
      ],
    },
    '400': {
      code: 'invalid_input',
      message: 'openLibraryId: malformed Open Library work ID',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'work not found on Open Library',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '503': {
      code: 'unavailable',
      message: 'Open Library is temporarily unavailable',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getDiscoverCover: {
    '400': {
      code: 'invalid_input',
      message: 'coverId: must be a positive integer',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'cover not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '503': {
      code: 'unavailable',
      message: 'Open Library Covers API is temporarily unavailable',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
} as const
