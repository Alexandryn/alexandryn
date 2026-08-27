import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { DataTable, type DataTableColumn } from './DataTable'

interface Book {
  id: string
  title: string
  author: string
}

const books: Book[] = [
  { id: '1', title: 'Dune', author: 'Herbert' },
  { id: '2', title: 'Foundation', author: 'Asimov' },
]

const columns: DataTableColumn<Book>[] = [
  { key: 'title', header: 'Title', render: (b) => b.title, sortable: true },
  { key: 'author', header: 'Author', render: (b) => b.author },
]

describe('DataTable — render', () => {
  it('renders real <table>/<th scope="col"> semantics, not a div-grid', () => {
    render(<DataTable caption="Books" columns={columns} rows={books} rowKey={(b) => b.id} />)
    const table = screen.getByRole('table', { name: 'Books' })
    expect(table.tagName).toBe('TABLE')
    const titleHeader = screen.getByRole('columnheader', { name: /Title/ })
    expect(titleHeader.tagName).toBe('TH')
    expect(titleHeader).toHaveAttribute('scope', 'col')
  })

  it('renders one row per data row with the rendered cell content', () => {
    render(<DataTable caption="Books" columns={columns} rows={books} rowKey={(b) => b.id} />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('Foundation')).toBeInTheDocument()
    expect(screen.getAllByRole('row')).toHaveLength(3) // header + 2 data rows
  })

  it('has zero axe violations', async () => {
    const { container } = render(
      <DataTable caption="Books" columns={columns} rows={books} rowKey={(b) => b.id} />,
    )
    expectNoAxeViolations(await runAxe(container))
  })
})

describe('DataTable — sort', () => {
  it('reflects sort state via aria-sort and is keyboard-operable', async () => {
    const onSortChange = vi.fn()
    const user = userEvent.setup()
    render(
      <DataTable
        caption="Books"
        columns={columns}
        rows={books}
        rowKey={(b) => b.id}
        sort={{ columnKey: 'title', direction: 'asc' }}
        onSortChange={onSortChange}
      />,
    )
    const titleHeader = screen.getByRole('columnheader', { name: /Title/ })
    expect(titleHeader).toHaveAttribute('aria-sort', 'ascending')
    const authorHeader = screen.getByRole('columnheader', { name: 'Author' })
    expect(authorHeader).not.toHaveAttribute('aria-sort')

    const sortButton = screen.getByRole('button', { name: /Title/ })
    await user.tab()
    expect(sortButton).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onSortChange).toHaveBeenCalledWith('title')
  })
})

describe('DataTable — select', () => {
  it('announces the selected row via aria-selected', () => {
    render(
      <DataTable
        caption="Books"
        columns={columns}
        rows={books}
        rowKey={(b) => b.id}
        selectedRowKeys={new Set(['1'])}
        onRowSelect={vi.fn()}
      />,
    )
    const rows = screen.getAllByRole('row')
    expect(rows[1]).toHaveAttribute('aria-selected', 'true')
    expect(rows[2]).toHaveAttribute('aria-selected', 'false')
  })

  it('is keyboard-operable (Tab + Enter selects a row)', async () => {
    const onRowSelect = vi.fn()
    const user = userEvent.setup()
    render(
      <DataTable
        caption="Books"
        columns={columns}
        rows={books}
        rowKey={(b) => b.id}
        onRowSelect={onRowSelect}
      />,
    )
    // The Title column is sortable, so its header button is the first tab stop.
    await user.tab()
    await user.tab()
    expect(screen.getAllByRole('row')[1]).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onRowSelect).toHaveBeenCalledWith('1')
  })

  it('rows are a roving tab stop: Arrow keys move between them, one Tab leaves the table (FR-1)', async () => {
    const onRowSelect = vi.fn()
    const user = userEvent.setup()
    render(
      <>
        <DataTable
          caption="Books"
          columns={columns}
          rows={books}
          rowKey={(b) => b.id}
          onRowSelect={onRowSelect}
        />
        <button type="button">after</button>
      </>,
    )
    const dataRows = screen.getAllByRole('row').slice(1)

    // Only the first data row is a tab stop; the rest are removed from the sequence.
    expect(dataRows[0]).toHaveAttribute('tabindex', '0')
    expect(dataRows[1]).toHaveAttribute('tabindex', '-1')

    await user.tab() // header sort button
    await user.tab() // first data row
    expect(dataRows[0]).toHaveFocus()

    await user.keyboard('{ArrowDown}')
    expect(dataRows[1]).toHaveFocus()
    expect(dataRows[1]).toHaveAttribute('tabindex', '0')
    expect(dataRows[0]).toHaveAttribute('tabindex', '-1')

    await user.keyboard('{ArrowUp}')
    expect(dataRows[0]).toHaveFocus()

    await user.keyboard('{End}')
    expect(dataRows[1]).toHaveFocus()

    await user.keyboard('{Home}')
    expect(dataRows[0]).toHaveFocus()

    // Selection still works on the focused row.
    await user.keyboard('{Enter}')
    expect(onRowSelect).toHaveBeenCalledWith('1')

    // A further Tab leaves the table entirely.
    await user.tab()
    expect(screen.getByRole('button', { name: 'after' })).toHaveFocus()
  })

  it('has zero axe violations with selection enabled', async () => {
    const { container } = render(
      <DataTable
        caption="Books"
        columns={columns}
        rows={books}
        rowKey={(b) => b.id}
        selectedRowKeys={new Set(['1'])}
        onRowSelect={vi.fn()}
      />,
    )
    expectNoAxeViolations(await runAxe(container))
  })

  it('does not fire row selection when a nested interactive control inside a cell is activated', async () => {
    const onRowSelect = vi.fn()
    const onDelete = vi.fn()
    const user = userEvent.setup()
    const columnsWithAction: DataTableColumn<Book>[] = [
      ...columns,
      { key: 'action', header: 'Action', render: () => <button onClick={onDelete}>Delete</button> },
    ]
    render(
      <DataTable
        caption="Books"
        columns={columnsWithAction}
        rows={books}
        rowKey={(b) => b.id}
        onRowSelect={onRowSelect}
      />,
    )
    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0]!)
    expect(onDelete).toHaveBeenCalledTimes(1)
    expect(onRowSelect).not.toHaveBeenCalled()

    // Clicking the row itself (not a nested control) still selects it.
    await user.click(screen.getByText('Herbert'))
    expect(onRowSelect).toHaveBeenCalledWith('1')
  })
})
