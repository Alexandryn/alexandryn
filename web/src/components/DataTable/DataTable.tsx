import type { KeyboardEvent, MouseEvent, ReactNode, TableHTMLAttributes } from 'react'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

const INTERACTIVE_SELECTOR = 'button, a, input, select, textarea, [role="button"], [role="link"]'

/** True when the event originated on an interactive descendant of the row, not the row itself. */
function isFromNestedInteractiveElement(target: EventTarget | null, row: HTMLElement): boolean {
  if (!(target instanceof HTMLElement)) return false
  const closest = target.closest(INTERACTIVE_SELECTOR)
  return closest !== null && closest !== row
}

export interface DataTableColumn<T> {
  key: string
  header: string
  render: (row: T) => ReactNode
  sortable?: boolean
}

export interface DataTableSort {
  columnKey: string
  direction: 'asc' | 'desc'
}

export interface DataTableProps<T> extends Omit<TableHTMLAttributes<HTMLTableElement>, 'children'> {
  columns: DataTableColumn<T>[]
  rows: T[]
  rowKey: (row: T) => string
  caption?: string
  sort?: DataTableSort
  onSortChange?: (columnKey: string) => void
  selectedRowKeys?: ReadonlySet<string>
  onRowSelect?: (rowKey: string) => void
}

/**
 * Hand-built, not Radix-wrapped — Radix ships no table primitive (FR-1).
 * Real <table>/<th scope> semantics throughout; sort and row selection are
 * implemented directly against them rather than a div-grid.
 */
export function DataTable<T>({
  columns,
  rows,
  rowKey,
  caption,
  sort,
  onSortChange,
  selectedRowKeys,
  onRowSelect,
  className,
  ...rest
}: DataTableProps<T>) {
  function handleRowClick(event: MouseEvent<HTMLTableRowElement>, key: string) {
    if (!onRowSelect) return
    if (isFromNestedInteractiveElement(event.target, event.currentTarget)) return
    onRowSelect(key)
  }

  function handleRowKeyDown(event: KeyboardEvent<HTMLTableRowElement>, key: string) {
    if (!onRowSelect) return
    if (isFromNestedInteractiveElement(event.target, event.currentTarget)) return
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      onRowSelect(key)
    }
  }

  return (
    <table className={cx('w-full border-collapse text-sm', className)} {...rest}>
      {caption && (
        <caption className="text-left text-xs text-text-2 font-ui pb-xs">{caption}</caption>
      )}
      <thead>
        <tr>
          {columns.map((column) => {
            const isSorted = sort?.columnKey === column.key
            const ariaSort = isSorted
              ? sort.direction === 'asc'
                ? 'ascending'
                : 'descending'
              : undefined
            return (
              <th
                key={column.key}
                scope="col"
                aria-sort={column.sortable ? (ariaSort ?? 'none') : undefined}
                className="text-left text-xs text-text-2 font-ui border-b border-border px-sm py-2xs"
              >
                {column.sortable ? (
                  <button
                    type="button"
                    onClick={() => onSortChange?.(column.key)}
                    className={cx('inline-flex items-center gap-4xs', FOCUS_RING)}
                  >
                    {column.header}
                    <span aria-hidden="true">
                      {isSorted ? (sort.direction === 'asc' ? '▲' : '▼') : ''}
                    </span>
                  </button>
                ) : (
                  column.header
                )}
              </th>
            )
          })}
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => {
          const key = rowKey(row)
          const selected = selectedRowKeys?.has(key) ?? false
          return (
            <tr
              key={key}
              aria-selected={onRowSelect ? selected : undefined}
              tabIndex={onRowSelect ? 0 : undefined}
              onClick={onRowSelect ? (event) => handleRowClick(event, key) : undefined}
              onKeyDown={onRowSelect ? (event) => handleRowKeyDown(event, key) : undefined}
              className={cx(
                'border-b border-border-2',
                onRowSelect && cx('cursor-pointer', FOCUS_RING),
                selected && 'bg-accent-soft',
              )}
            >
              {columns.map((column) => (
                <td key={column.key} className="px-sm py-2xs text-text">
                  {column.render(row)}
                </td>
              ))}
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}
