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
  listSources: {
    '200': {
      sources: [
        {
          id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
          label: 'Personal OPDS',
          kind: 'opds',
          config: {
            baseUrl: 'https://opds.example.org/catalog',
          },
          hasCredential: true,
          health: {
            status: 'reachable',
            checkedAt: '2026-08-31T12:00:00Z',
            detail: null,
          },
          capabilities: {
            canList: true,
            canSearch: true,
            canDownload: true,
          },
        },
      ],
    },
  },
  createSource: {
    '201': {
      id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
      label: 'Personal OPDS',
      kind: 'opds',
      config: {
        baseUrl: 'https://opds.example.org/catalog',
      },
      hasCredential: true,
      health: {
        status: 'reachable',
        checkedAt: '2026-08-31T12:00:00Z',
        detail: null,
      },
      capabilities: {
        canList: true,
        canSearch: false,
        canDownload: true,
      },
    },
    '400': {
      code: 'invalid_input',
      message: 'kind: must be one of local-folder, opds',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getSource: {
    '200': {
      id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
      label: 'Studio NAS',
      kind: 'local-folder',
      config: {
        basePath: '/srv/books',
      },
      hasCredential: false,
      health: {
        status: 'reachable',
        checkedAt: '2026-08-31T12:00:00Z',
        detail: null,
      },
      capabilities: {
        canList: true,
        canSearch: false,
        canDownload: true,
      },
    },
    '404': {
      code: 'not_found',
      message: 'source not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  deleteSource: {
    '404': {
      code: 'not_found',
      message: 'source not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  updateSource: {
    '200': {
      id: '01JXXXXXXXXXXXXXXXXXXXXXXZ',
      label: 'Personal OPDS (renamed)',
      kind: 'opds',
      config: {
        baseUrl: 'https://opds.example.org/catalog',
      },
      hasCredential: true,
      health: {
        status: 'reachable',
        checkedAt: '2026-08-31T12:05:00Z',
        detail: null,
      },
      capabilities: {
        canList: true,
        canSearch: true,
        canDownload: true,
      },
    },
    '400': {
      code: 'invalid_input',
      message: 'config.baseUrl: must be an http or https URL',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'source not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  checkSourceHealth: {
    '200': {
      status: 'unreachable',
      checkedAt: '2026-08-31T12:10:00Z',
      detail: 'timeout',
    },
    '404': {
      code: 'not_found',
      message: 'source not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  browseSource: {
    '200': {
      items: [
        {
          title: 'The Left Hand of Darkness',
          author: 'Ursula K. Le Guin',
          fileReference: {
            referenceId: 'left-hand-of-darkness.epub',
            format: 'EPUB',
            sizeBytes: 512000,
          },
          coverUrl: null,
        },
      ],
      nextCursor: null,
    },
    '400': {
      code: 'invalid_input',
      message: 'limit: must be an integer between 1 and 50',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'source not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '503': {
      code: 'unavailable',
      message: 'source is unavailable right now',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  searchSource: {
    '200': {
      items: [
        {
          title: 'A Wizard of Earthsea',
          author: 'Ursula K. Le Guin',
          fileReference: {
            referenceId: 'wizard-of-earthsea.epub',
            format: 'EPUB',
            sizeBytes: null,
          },
          coverUrl: 'https://opds.example.org/covers/earthsea.jpg',
        },
      ],
      nextCursor: 'eyJwIjoyfQ',
    },
    '400': {
      code: 'invalid_input',
      message: 'q: search query must be non-empty',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'source not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '409': {
      code: 'conflict',
      message: 'this source does not support search',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '503': {
      code: 'unavailable',
      message: 'source is unavailable right now',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  discoverImport: {
    '202': {
      discoveredCount: 5,
      skippedCount: 1,
      jobIds: ['01JJOB1', '01JJOB2'],
    },
    '400': {
      code: 'invalid_input',
      message: 'source does not support listing',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'source not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  listImportCandidates: {
    '200': {
      candidates: [
        {
          id: '01JCAND1',
          sourceId: '01JSRC1',
          fileReference: {
            id: 'dune.epub',
            format: 'epub',
            sizeBytes: 1048576,
          },
          status: 'pending',
          extractedMetadata: {
            title: 'Dune',
            authors: ['Frank Herbert'],
            isbn: '9780441172719',
          },
          matchCandidates: [
            {
              type: 'open_library_work',
              confidence: 'high',
              title: 'Dune',
              author: 'Frank Herbert',
              openLibraryWorkKey: 'OL893415W',
            },
          ],
          createdAt: '2026-09-01T12:00:00Z',
          updatedAt: '2026-09-01T12:00:00Z',
        },
      ],
    },
    '400': {
      code: 'invalid_input',
      message: 'invalid status filter',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  confirmImportCandidate: {
    '200': {
      id: '01JCAND1',
      sourceId: '01JSRC1',
      fileReference: {
        id: 'dune.epub',
        format: 'epub',
        sizeBytes: 1048576,
      },
      status: 'confirmed',
      createdAt: '2026-09-01T12:00:00Z',
      updatedAt: '2026-09-01T12:00:00Z',
    },
    '400': {
      code: 'invalid_input',
      message: 'invalid action',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'candidate not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '409': {
      code: 'conflict',
      message: 'candidate is not pending',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  rejectImportCandidate: {
    '200': {
      id: '01JCAND1',
      sourceId: '01JSRC1',
      fileReference: {
        id: 'dune.epub',
        format: 'epub',
        sizeBytes: 1048576,
      },
      status: 'rejected',
      createdAt: '2026-09-01T12:00:00Z',
      updatedAt: '2026-09-01T12:00:00Z',
    },
    '404': {
      code: 'not_found',
      message: 'candidate not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '409': {
      code: 'conflict',
      message: 'candidate is not pending',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getReaderContent: {
    '400': {
      code: 'invalid_input',
      message: 'resource path must be relative',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'no such resource in this book',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '503': {
      code: 'unavailable',
      message: "this book's source isn't reachable right now",
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getReadingProgress: {
    '200': {
      progress: null,
    },
  },
  reportReadingProgress: {
    '200': {
      progress: {
        percentage: 0.42,
        epoch: 0,
        precisePosition: null,
        observedAt: '2026-09-01T12:00:00Z',
      },
      outcome: 'advanced',
    },
    '400': {
      code: 'invalid_input',
      message: 'X-Device-Id must be a version 4 UUID',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  listBookmarks: {
    '200': {
      bookmarks: [],
    },
  },
  createBookmark: {
    '201': {
      bookmark: {
        id: 'bmk_1',
        editionId: 'edn_1',
        cfi: 'epubcfi(/6/4!/10)',
        label: 'the turn',
        createdAt: '2026-09-01T12:00:00Z',
      },
    },
    '400': {
      code: 'invalid_input',
      message: 'position is not an epubcfi(...) value',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  deleteBookmark: {
    '404': {
      code: 'not_found',
      message: 'bookmark not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  listHighlights: {
    '200': {
      highlights: [],
    },
  },
  createHighlight: {
    '201': {
      highlight: {
        id: 'hlt_1',
        editionId: 'edn_1',
        startCfi: 'epubcfi(/6/4!/10/1:0)',
        endCfi: 'epubcfi(/6/4!/10/1:88)',
        note: '',
        category: 'blue',
        createdAt: '2026-09-01T12:00:00Z',
      },
    },
    '400': {
      code: 'invalid_input',
      message: 'endCfi must not sort before startCfi',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  deleteHighlight: {
    '404': {
      code: 'not_found',
      message: 'highlight not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  patchHighlight: {
    '200': {
      highlight: {
        id: 'hlt_1',
        editionId: 'edn_1',
        startCfi: 'epubcfi(/6/4!/10/1:0)',
        endCfi: 'epubcfi(/6/4!/10/1:88)',
        note: 'a thought',
        category: 'blue',
        createdAt: '2026-09-01T12:00:00Z',
      },
    },
    '400': {
      code: 'invalid_input',
      message: 'note: too long',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'highlight not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getReadingPreferences: {
    '200': {
      preferences: {
        font: 'serif',
        fontSize: 19,
        lineSpacing: 1.5,
        theme: 'light',
        layoutMode: 'paginated',
        columnWidth: 'default',
      },
    },
  },
  putReadingPreferences: {
    '200': {
      preferences: {
        font: 'Newsreader',
        fontSize: 22,
        lineSpacing: 1.7,
        theme: 'sepia',
        layoutMode: 'scroll',
        columnWidth: 'wide',
      },
    },
    '400': {
      code: 'invalid_input',
      message: 'theme must be light, sepia, or dark',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  exportReadingData: {
    '200': {
      schemaVersion: 1,
      exportedAt: '2026-09-01T12:00:00Z',
      works: [],
      editions: [],
    },
    '404': {
      code: 'not_found',
      message: 'no work with that id',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getSetupStatus: {
    '200': {
      isSetup: false,
    },
  },
  setupAdmin: {
    '201': {
      user: {
        id: '00000000-0000-0000-0000-000000000001',
        username: 'librarian',
        email: 'admin@alexandryn.org',
        role: 'admin',
      },
      accessToken: 'mock.jwt.token',
      refreshToken: 'mock-refresh-token',
    },
    '409': {
      code: 'conflict',
      message: 'system is already initialized',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  login: {
    '200': {
      user: {
        id: '00000000-0000-0000-0000-000000000001',
        username: 'librarian',
        email: 'admin@alexandryn.org',
        role: 'admin',
      },
      accessToken: 'mock.jwt.token',
      refreshToken: 'mock-refresh-token',
    },
    '401': {
      code: 'unauthorized',
      message: 'invalid username or password',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  refreshToken: {
    '200': {
      accessToken: 'mock.jwt.token2',
      refreshToken: 'mock-refresh-token2',
    },
    '401': {
      code: 'unauthorized',
      message: 'invalid refresh token',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  requestPasswordReset: {
    '200': {
      message: 'if the email is registered, a password reset link has been dispatched',
    },
  },
  confirmPasswordReset: {
    '200': {
      success: true,
    },
  },
  setupTOTP: {
    '200': {
      secret: 'JBSWY3DPEHPK3PXP',
      keyUri: 'otpauth://totp/Alexandryn:librarian?secret=JBSWY3DPEHPK3PXP&issuer=Alexandryn',
      recoveryCodes: ['ABCD-1234-EF', '5678-GHIJ-90'],
    },
  },
  confirmTOTP: {
    '200': {
      enabled: true,
    },
  },
  verifyTOTP: {
    '200': {
      user: {
        id: '00000000-0000-0000-0000-000000000001',
        username: 'librarian',
        email: 'admin@alexandryn.org',
        role: 'admin',
      },
      accessToken: 'mock.jwt.token',
      refreshToken: 'mock-refresh-token',
    },
  },
  disableTOTP: {
    '200': {
      disabled: true,
    },
  },
  listLibraries: {
    '200': {
      libraries: [
        {
          id: '00000000-0000-0000-0000-000000000001',
          name: 'Default Library',
          description: 'Main library namespace',
          allowReaderUploads: false,
          createdAt: '2026-09-01T12:00:00Z',
          updatedAt: '2026-09-01T12:00:00Z',
        },
      ],
    },
  },
  createLibrary: {
    '201': {
      library: {
        id: '00000000-0000-0000-0000-000000000002',
        name: 'Comics',
        description: 'Comics and Manga',
        allowReaderUploads: true,
        createdAt: '2026-09-01T12:00:00Z',
        updatedAt: '2026-09-01T12:00:00Z',
      },
    },
  },
  getLibrary: {
    '200': {
      library: {
        id: '00000000-0000-0000-0000-000000000001',
        name: 'Default Library',
        description: 'Main library namespace',
        allowReaderUploads: false,
        createdAt: '2026-09-01T12:00:00Z',
        updatedAt: '2026-09-01T12:00:00Z',
      },
    },
  },
  updateLibrary: {
    '200': {
      library: {
        id: '00000000-0000-0000-0000-000000000001',
        name: 'Default Library Updated',
        description: 'Main library namespace',
        allowReaderUploads: true,
        createdAt: '2026-09-01T12:00:00Z',
        updatedAt: '2026-09-01T12:00:00Z',
      },
    },
  },
  listLibraryMembers: {
    '200': {
      members: [
        {
          id: '00000000-0000-0000-0000-000000000001',
          userId: '00000000-0000-0000-0000-000000000001',
          username: 'librarian',
          email: 'admin@alexandryn.org',
          role: 'admin',
          createdAt: '2026-09-01T12:00:00Z',
        },
      ],
    },
  },
  createLibraryInvitation: {
    '201': {
      invitationToken: 'mock-invite-token',
      invitationUrl: '/invite/mock-invite-token',
      expiresAt: '2026-09-08T12:00:00Z',
    },
  },
  acceptLibraryInvitation: {
    '200': {
      libraryId: '00000000-0000-0000-0000-000000000001',
      role: 'reader',
    },
  },
  initiatePairing: {
    '201': {
      pairingId: '00000000-0000-0000-0000-000000000001',
      code: 'ABCD-EFGH',
      payload: 'http://192.168.1.50:8080/connect?c=ABCD-EFGH',
      address: '192.168.1.50:8080',
      expiresAt: '2026-09-05T12:05:00Z',
    },
    '400': {
      code: 'invalid_input',
      message: 'malformed request body or body too large',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '403': {
      code: 'forbidden',
      message: 'insufficient permissions for this resource',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '415': {
      code: 'unsupported_media_type',
      message: 'Content-Type must be application/json',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '429': {
      code: 'rate_limited',
      message: 'too many requests, slow down and try again shortly',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  verifyPairing: {
    '200': {
      enrolmentGrant: 'mock.enrolment.jwt',
      address: '192.168.1.50:8080',
      hostName: 'alexandryn.local',
    },
    '400': {
      code: 'invalid_input',
      message: 'malformed pairing code: must be 8 Crockford base32 characters',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '403': {
      code: 'forbidden',
      message: 'request origin is not allowed',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'pairing code not recognised',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '415': {
      code: 'unsupported_media_type',
      message: 'Content-Type must be application/json',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '429': {
      code: 'rate_limited',
      message: 'too many requests, slow down and try again shortly',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getPairingQR: {
    '200': {
      address: '192.168.1.50:8080',
      expiresAt: '2026-09-05T12:05:00Z',
      state: 'pending',
      code: 'ABCD-EFGH',
      payload: 'http://192.168.1.50:8080/connect?c=ABCD-EFGH',
    },
    '403': {
      code: 'forbidden',
      message: 'insufficient permissions for this resource',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'pairing session not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  getNetworkStatus: {
    '200': {
      reachability: 'lan',
      tlsMode: 'none',
      authRequired: true,
      address: 'http://192.168.1.50:8080',
      addresses: [
        {
          scope: 'lan',
          url: 'http://192.168.1.50:8080',
        },
      ],
      hostName: 'alexandryn.local',
    },
    '401': {
      code: 'unauthorized',
      message: 'missing or invalid authorization token',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  updateNetworkSettings: {
    '200': {
      hostName: 'alexandryn.local',
      rememberDeviceDays: 30,
      updatedAt: '2026-09-05T12:00:00Z',
    },
    '400': {
      code: 'invalid_input',
      message: 'unrecognised settings key "unknownKey"',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '403': {
      code: 'forbidden',
      message: 'insufficient permissions for this resource',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '415': {
      code: 'unsupported_media_type',
      message: 'Content-Type must be application/json',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
  deletePairing: {
    '403': {
      code: 'forbidden',
      message: 'insufficient permissions for this resource',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
    '404': {
      code: 'not_found',
      message: 'pairing session not found',
      correlationId: '00000000-0000-0000-0000-000000000000',
    },
  },
} as const
