import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { DiscoverResultGrid } from './DiscoverResultGrid'
import { DiscoverCover } from './DiscoverCover'
import type { NormalisedSearchResult } from '../../data/discover'

describe('DiscoverResultGrid & DiscoverCover (FR-1, FR-4, FR-7)', () => {
  const sampleResults: NormalisedSearchResult[] = [
    {
      openLibraryWorkKey: 'OL82563W',
      title: 'Middlemarch',
      authors: [{ name: 'George Eliot' }],
      firstPublishYear: 1871,
      coverUrl: '/api/v1/discover/covers/8256301',
      editionCount: 42,
    },
    {
      openLibraryWorkKey: 'OL99999W',
      title: 'Book Without Cover',
      authors: [{ name: 'Author A' }, { name: 'Author B' }],
      coverUrl: null,
      editionCount: 1,
    },
  ]

  it('renders search results grid with links and metadata', () => {
    render(
      <MemoryRouter>
        <DiscoverResultGrid results={sampleResults} />
      </MemoryRouter>,
    )

    expect(screen.getAllByText('Middlemarch').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('George Eliot')).toBeInTheDocument()
    expect(screen.getByText('1871')).toBeInTheDocument()
    expect(screen.getByText('42 editions')).toBeInTheDocument()

    expect(screen.getAllByText('Book Without Cover').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Author A, Author B')).toBeInTheDocument()
    expect(screen.getByText('1 edition')).toBeInTheDocument()

    const links = screen.getAllByRole('link')
    expect(links[0]).toHaveAttribute('href', '/discover/works/OL82563W')
    expect(links[1]).toHaveAttribute('href', '/discover/works/OL99999W')
  })

  it('DiscoverCover renders network image when coverUrl is provided', () => {
    const { container } = render(
      <DiscoverCover coverUrl="/api/v1/discover/covers/123" identifier="OL1W" title="Test Title" />,
    )

    const img = container.querySelector('img')
    expect(img).not.toBeNull()
    expect(img).toHaveAttribute('src', '/api/v1/discover/covers/123')
    expect(img).toHaveAttribute('loading', 'lazy')
  })

  it('DiscoverCover falls back to GeneratedCover on image load error (FR-7)', () => {
    const { container } = render(
      <DiscoverCover
        coverUrl="/api/v1/discover/covers/broken"
        identifier="OL1W"
        title="Test Title"
      />,
    )

    const img = container.querySelector('img')
    expect(img).not.toBeNull()

    // Trigger onError
    fireEvent.error(img!)

    // Now img should be replaced with GeneratedCover
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getByText('Test Title')).toBeInTheDocument()
  })

  it('renders priority hint with eager loading and high fetchPriority on first row (audit 0016 #219)', () => {
    const { container } = render(
      <DiscoverCover
        coverUrl="/api/v1/discover/covers/123"
        identifier="OL1W"
        title="Test Title"
        priority
      />,
    )

    const img = container.querySelector('img')
    expect(img).not.toBeNull()
    expect(img).toHaveAttribute('loading', 'eager')
    expect(img).toHaveAttribute('fetchpriority', 'high')
    expect(img).toHaveAttribute('decoding', 'async')
  })

  it('passes priority=true for first 6 results in DiscoverResultGrid (audit 0016 #219)', () => {
    const manyResults: NormalisedSearchResult[] = Array.from({ length: 8 }, (_, i) => ({
      openLibraryWorkKey: `OL${i}W`,
      title: `Book ${i}`,
      authors: [{ name: `Author ${i}` }],
      coverUrl: `/api/v1/discover/covers/${i}`,
      editionCount: 1,
    }))

    const { container } = render(
      <MemoryRouter>
        <DiscoverResultGrid results={manyResults} />
      </MemoryRouter>,
    )

    const images = container.querySelectorAll('img')
    expect(images.length).toBe(8)
    // First 6 covers are priority
    for (let i = 0; i < 6; i++) {
      expect(images[i]).toHaveAttribute('loading', 'eager')
      expect(images[i]).toHaveAttribute('fetchpriority', 'high')
    }
    // 7th and 8th covers are lazy
    expect(images[6]).toHaveAttribute('loading', 'lazy')
    expect(images[7]).toHaveAttribute('loading', 'lazy')
  })
})
