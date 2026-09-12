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

describe('SourceFormDialog', () => {
  it('renders Add source form fields', () => {
    renderWithQueryClient(<SourceFormDialog open={true} onOpenChange={vi.fn()} />)

    expect(screen.getByRole('heading', { name: 'Add source' })).toBeInTheDocument()
    expect(screen.getByLabelText('Source label')).toBeInTheDocument()
    expect(screen.getByText('Local folder')).toBeInTheDocument()
    expect(screen.getByText('OPDS catalog')).toBeInTheDocument()
    expect(screen.getByLabelText('Folder path')).toBeInTheDocument()
  })

  it('renders Browse... button only when window.alexandryn is available', () => {
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

  it('shows HTTP warning when non-HTTPS URL has credential form open', async () => {
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

  it('renders Password set static text in edit mode with Replace action', async () => {
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

  it('ties validation error to specific input with aria-invalid and moves focus', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<SourceFormDialog open={true} onOpenChange={vi.fn()} />)

    // Submit with empty label
    const submitBtn = screen.getByRole('button', { name: 'Add source' })
    await user.click(submitBtn)

    const labelInput = screen.getByLabelText('Source label')
    expect(labelInput).toHaveAttribute('aria-invalid', 'true')
    expect(labelInput).toHaveFocus()
    const errorId = labelInput.getAttribute('aria-describedby')
    expect(errorId).toBeTruthy()
    const errorMsg = document.getElementById(errorId!)
    expect(errorMsg).toHaveTextContent('Source label is required.')

    // Type valid label, leaving folder path empty
    await user.type(labelInput, 'My Library')
    await user.click(submitBtn)

    const folderInput = screen.getByLabelText('Folder path')
    expect(folderInput).toHaveAttribute('aria-invalid', 'true')
    expect(folderInput).toHaveFocus()
  })
})
