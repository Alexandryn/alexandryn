import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { WorkGrid } from './WorkGrid'
import type { WorkSummary } from '../../data/library'

function makeMockWorks(count: number): WorkSummary[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `work-${i + 1}`,
    title: `Book Title ${i + 1}`,
    subtitle: i % 2 === 0 ? `Subtitle ${i + 1}` : '',
    authors: [`Author ${i + 1}`],
    isOwned: i % 3 !== 0,
    collections: i % 2 === 0 ? [{ id: 'coll-1', name: 'Favorites' }] : [],
    addedAt: '2026-01-15T10:00:00Z',
  }))
}

describe('WorkGrid', () => {
  it('renders grid view correctly for <= 100 items without placeholders', () => {
    const works = makeMockWorks(10)
    render(
      <MemoryRouter>
        <WorkGrid works={works} view="grid" />
      </MemoryRouter>,
    )

    expect(screen.getAllByText('Book Title 1').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Book Title 10').length).toBeGreaterThan(0)
    expect(screen.queryAllByTestId('virtual-cover-placeholder')).toHaveLength(0)
  })

  it('renders list view with subtitles and collection badges', () => {
    const works = makeMockWorks(5)
    render(
      <MemoryRouter>
        <WorkGrid works={works} view="list" />
      </MemoryRouter>,
    )

    expect(screen.getAllByText('Book Title 1').length).toBeGreaterThan(0)
    expect(screen.getByText('Subtitle 1')).toBeInTheDocument()
    expect(screen.getAllByText('Favorites').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Wanted').length).toBeGreaterThan(0)

  })

  it('virtualizes when items exceed 100 threshold', () => {
    const works = makeMockWorks(120)
    render(
      <MemoryRouter>
        <WorkGrid works={works} view="grid" />
      </MemoryRouter>,
    )

    // Above threshold (120 > 100), items beyond initial visible count (40) render placeholders
    const placeholders = screen.getAllByTestId('virtual-cover-placeholder')
    expect(placeholders.length).toBe(120 - 40)
  })
})
