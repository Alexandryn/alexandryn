import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it } from 'vitest'

import { server } from '../../mocks/node'
import { LibrarySwitcher } from './LibrarySwitcher'

const twoLibraries = {
  libraries: [
    { id: 'lib-1', name: 'Fiction', description: '', allowReaderUploads: false, createdAt: '', updatedAt: '' },
    { id: 'lib-2', name: 'History', description: '', allowReaderUploads: false, createdAt: '', updatedAt: '' },
  ],
}

describe('LibrarySwitcher (audit 0016 #138)', () => {
  afterEach(() => localStorage.clear())

  it('evicts every library-scoped query on switch and keeps the rest', async () => {
    server.use(http.get('*/api/v1/libraries', () => HttpResponse.json(twoLibraries)))
    localStorage.setItem('alexandryn_active_library', 'lib-1')

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    // Prime the cache: some scoped to a library, some not.
    qc.setQueryData(['work', 'w-1'], { id: 'w-1' })
    qc.setQueryData(['sources', 'list'], [])
    qc.setQueryData(['collections'], [])
    qc.setQueryData(['activity', 'events'], [])
    qc.setQueryData(['reading', 'progress', 'w-1'], {})
    qc.setQueryData(['libraries'], twoLibraries)
    qc.setQueryData(['devices'], [])
    qc.setQueryData(['discover', 'search', { q: 'dune' }], {})

    render(
      <QueryClientProvider client={qc}>
        <LibrarySwitcher />
      </QueryClientProvider>,
    )

    const select = await screen.findByLabelText('Active Library:')
    await userEvent.selectOptions(select, 'lib-2')

    await waitFor(() => expect(localStorage.getItem('alexandryn_active_library')).toBe('lib-2'))

    const invalidated = (key: unknown[]) => qc.getQueryState(key)?.isInvalidated === true
    for (const key of [
      ['work', 'w-1'],
      ['sources', 'list'],
      ['collections'],
      ['activity', 'events'],
      ['reading', 'progress', 'w-1'],
    ]) {
      expect(invalidated(key), `expected ${JSON.stringify(key)} invalidated`).toBe(true)
    }
    for (const key of [['libraries'], ['devices'], ['discover', 'search', { q: 'dune' }]]) {
      expect(invalidated(key), `expected ${JSON.stringify(key)} untouched`).toBe(false)
    }
  })
})
