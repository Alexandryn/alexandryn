import { Provider as ToastProvider } from '@radix-ui/react-toast'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Button } from '../components/Button/Button'
import { DataTable, type DataTableColumn } from '../components/DataTable/DataTable'
import { Modal } from '../components/Modal/Modal'
import { SegmentedControl } from '../components/SegmentedControl/SegmentedControl'
import { Slider } from '../components/Slider/Slider'
import { Toast, ToastViewport } from '../components/Toast/Toast'
import { Toggle } from '../components/Toggle/Toggle'

// Holds web/docs/keyboard-map.md to the code (frontend-accessibility.md
// FR-1 / Acceptance criterion 2): one representative primitive per key
// category, keyboard only — never a pointer event.

describe('keyboard map — Tab / Shift+Tab move focus in DOM order', () => {
  it('walks forward and backward through interactive elements', async () => {
    const user = userEvent.setup()
    render(
      <>
        <Button>first</Button>
        <Button>second</Button>
        <Button>third</Button>
      </>,
    )
    await user.tab()
    expect(screen.getByRole('button', { name: 'first' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: 'second' })).toHaveFocus()
    await user.tab({ shift: true })
    expect(screen.getByRole('button', { name: 'first' })).toHaveFocus()
  })
})

describe('keyboard map — Enter / Space activate the focused control', () => {
  it('Button fires on both Enter and Space', async () => {
    const onClick = vi.fn()
    const user = userEvent.setup()
    render(<Button onClick={onClick}>go</Button>)
    await user.tab()
    await user.keyboard('{Enter}')
    await user.keyboard(' ')
    expect(onClick).toHaveBeenCalledTimes(2)
  })

  it('Toggle flips on Space', async () => {
    const onCheckedChange = vi.fn()
    const user = userEvent.setup()
    render(<Toggle label="Notifications" onCheckedChange={onCheckedChange} />)
    await user.tab()
    await user.keyboard(' ')
    expect(onCheckedChange).toHaveBeenCalledWith(true)
  })
})

describe('keyboard map — Arrow keys move within a composite control (roving)', () => {
  it('SegmentedControl: Arrow keys move the selection between segments', async () => {
    const onValueChange = vi.fn()
    const user = userEvent.setup()
    render(
      <SegmentedControl
        aria-label="View"
        defaultValue="grid"
        onValueChange={onValueChange}
        options={[
          { value: 'grid', label: 'Grid' },
          { value: 'list', label: 'List' },
        ]}
      />,
    )
    await user.tab()
    expect(screen.getByRole('radio', { name: 'Grid' })).toHaveFocus()
    // Radix commits the roving move on a macrotask only if the key is
    // still held — hold it, matching real key timing.
    await user.keyboard('{ArrowRight>}')
    await waitFor(() => expect(onValueChange).toHaveBeenCalledWith('list'))
    await user.keyboard('{/ArrowRight}')
  })

  it('Slider: Arrow keys adjust the value by one step', async () => {
    const onValueChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Slider
        label="Volume"
        defaultValue={[50]}
        min={0}
        max={100}
        step={1}
        onValueChange={onValueChange}
      />,
    )
    await user.tab()
    await user.keyboard('{ArrowRight}')
    expect(onValueChange).toHaveBeenCalledWith([51])
  })

  it('DataTable: Arrow keys move focus between rows (row-level roving)', async () => {
    const user = userEvent.setup()
    const columns: DataTableColumn<{ id: string; name: string }>[] = [
      { key: 'name', header: 'Name', render: (r) => r.name },
    ]
    render(
      <DataTable
        caption="People"
        columns={columns}
        rows={[
          { id: '1', name: 'Ada' },
          { id: '2', name: 'Grace' },
        ]}
        rowKey={(r) => r.id}
        onRowSelect={vi.fn()}
      />,
    )
    const rows = screen.getAllByRole('row').slice(1)
    await user.tab()
    expect(rows[0]).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(rows[1]).toHaveFocus()
  })
})

describe('keyboard map — Escape closes the topmost transient', () => {
  it('Modal closes on Escape', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(<Modal open title="Confirm" onOpenChange={onOpenChange} />)
    await user.keyboard('{Escape}')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('Toast dismisses on Escape', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(
      <ToastProvider>
        <Toast title="Saved" open duration={Infinity} onOpenChange={onOpenChange} />
        <ToastViewport />
      </ToastProvider>,
    )
    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument())
    await user.tab() // focus the toast surface
    await user.keyboard('{Escape}')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
