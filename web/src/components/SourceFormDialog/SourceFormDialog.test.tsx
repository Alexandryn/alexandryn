import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { SourceFormDialog } from './SourceFormDialog'
import type { Source } from '../../data/sources'

function renderWithQueryClient(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

interface AlexandrynWindow {
  alexandryn?: {
    source?: {
      pickLocalFolder?: () => Promise<{ path: string } | null>
    }
  }
}

describe('SourceFormDialog (FR-2, FR-4, FR-5)', () => {
  it('renders Add source form fields', () => {
    renderWithQueryClient(<SourceFormDialog open={true} onOpenChange={vi.fn()} />)

    expect(screen.getByRole('heading', { name: 'Add source' })).toBeInTheDocument()
    expect(screen.getByLabelText('Source label')).toBeInTheDocument()
    expect(screen.getByText('Local folder')).toBeInTheDocument()
    expect(screen.getByText('OPDS catalog')).toBeInTheDocument()
    expect(screen.getByLabelText('Folder path')).toBeInTheDocument()
  })

  it('renders Browse... button only when window.alexandryn is available (FR-4)', () => {
    const { unmount } = renderWithQueryClient(
      <SourceFormDialog open={true} onOpenChange={vi.fn()} />,
    )
    expect(screen.queryByRole('button', { name: 'Browse...' })).not.toBeInTheDocument()
    unmount()

    // Mock window.alexandryn
    const pickLocalFolder = vi.fn().mockResolvedValue({ path: '/home/user/books' })
    ;(window as unknown as AlexandrynWindow).alexandryn = { source: { pickLocalFolder } }

    renderWithQueryClient(<SourceFormDialog open={true} onOpenChange={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Browse...' })).toBeInTheDocument()

    delete (window as unknown as AlexandrynWindow).alexandryn
  })

  it('shows HTTP warning when non-HTTPS URL has credential form open (FR-5)', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<SourceFormDialog open={true} onOpenChange={vi.fn()} />)

    // Switch to OPDS
    await user.click(screen.getByText('OPDS catalog'))

    // Enable credentials
    await user.click(screen.getByRole('checkbox', { name: 'Requires a username and password' }))

    // Type http:// URL
    const urlInput = screen.getByLabelText('Catalog base URL')
    await user.type(urlInput, 'http://opds.insecure.org/feed')

    expect(
      screen.getByText(/This source doesn't use HTTPS. Your password will be sent unencrypted./i),
    ).toBeInTheDocument()
  })

  it('renders Password set static text in edit mode with Replace action (FR-5)', async () => {
    const user = userEvent.setup()
    const opdsSourceWithCred: Source = {
      id: 'src-1',
      label: 'Secure OPDS',
      kind: 'opds',
      config: { baseUrl: 'https://opds.example.com' },
      hasCredential: true,
      health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
      capabilities: { canList: true, canSearch: true, canDownload: true },
    }

    renderWithQueryClient(
      <SourceFormDialog open={true} onOpenChange={vi.fn()} source={opdsSourceWithCred} />,
    )

    expect(screen.getByRole('heading', { name: 'Edit source' })).toBeInTheDocument()
    expect(screen.getByText('Password set')).toBeInTheDocument()
    expect(screen.queryByLabelText('Username')).not.toBeInTheDocument()

    // Click Replace
    const replaceButton = screen.getByRole('button', { name: 'Replace' })
    await user.click(replaceButton)

    expect(screen.getByLabelText('Username')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
  })
})
