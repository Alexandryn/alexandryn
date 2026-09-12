import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { SourceCandidateList } from './SourceCandidateList'
import type { SourceCandidate } from '../../data/sources'

describe('SourceCandidateList', () => {
  const mockCandidates: SourceCandidate[] = [
    {
      title: 'The Left Hand of Darkness',
      author: 'Ursula K. Le Guin',
      fileReference: {
        referenceId: 'left-hand.epub',
        format: 'EPUB',
        sizeBytes: 512000,
      },
      coverUrl: null,
    },
    {
      title: 'A Wizard of Earthsea',
      author: 'Ursula K. Le Guin',
      fileReference: {
        referenceId: 'wizard.pdf',
        format: 'PDF',
        sizeBytes: 1048576,
      },
      coverUrl: 'https://example.com/cover.jpg',
    },
  ]

  it('renders candidate titles, authors, formats and sizes', () => {
    render(<SourceCandidateList candidates={mockCandidates} />)

    expect(
      screen.getByRole('heading', { name: 'The Left Hand of Darkness', level: 2 }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'A Wizard of Earthsea', level: 2 }),
    ).toBeInTheDocument()
    expect(screen.getByText('EPUB')).toBeInTheDocument()
    expect(screen.getByText('PDF')).toBeInTheDocument()
    expect(screen.getByText('500.0 KB')).toBeInTheDocument()
    expect(screen.getByText('1.0 MB')).toBeInTheDocument()
  })

  it('renders empty state when candidates array is empty', () => {
    render(<SourceCandidateList candidates={[]} />)
    expect(screen.getByText('No items found')).toBeInTheDocument()
  })

  it('renders loading state when isLoading is true', () => {
    render(<SourceCandidateList candidates={[]} isLoading={true} />)
    expect(screen.getByRole('status')).toBeInTheDocument()
    expect(screen.getByText('Loading items')).toBeInTheDocument()
  })

  it('renders error state when error is provided', () => {
    render(<SourceCandidateList candidates={[]} error={new Error('Failed to load')} />)
    expect(screen.getByText('Could not load books from source')).toBeInTheDocument()
  })

  it('renders load more button and handles click', async () => {
    const handleFetchNext = vi.fn()
    const user = userEvent.setup()

    render(
      <SourceCandidateList
        candidates={mockCandidates}
        hasNextPage={true}
        fetchNextPage={handleFetchNext}
      />,
    )

    const loadMoreButton = screen.getByRole('button', { name: 'Load more' })
    expect(loadMoreButton).toBeInTheDocument()

    await user.click(loadMoreButton)
    expect(handleFetchNext).toHaveBeenCalledTimes(1)
  })
})
