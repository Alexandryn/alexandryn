import {
  useRef,
  useState,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
  type TableHTMLAttributes,
} from 'react'
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
 * Hand-built, not Radix-wrapped — Radix ships no table primitive.
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
  // Row-level roving tabindex: when the
  // table is selectable, exactly one row is a tab stop and Arrow keys
  // move between rows. Row-level, not cell-level — table cells hold
  // no interactive content.
  const rowRefs = useRef<(HTMLTableRowElement | null)[]>([])
  const [activeRow, setActiveRow] = useState(0)
  // Clamp against the current row count so a shrinking `rows` prop can't
  // leave the table with no tab stop.
  const rovingRow = Math.max(0, Math.min(activeRow, rows.length - 1))

  function focusRow(index: number) {
    const clamped = Math.max(0, Math.min(index, rows.length - 1))
    setActiveRow(clamped)
    rowRefs.current[clamped]?.focus()
  }

  function handleRowClick(event: MouseEvent<HTMLTableRowElement>, key: string) {
    if (!onRowSelect) return
    if (isFromNestedInteractiveElement(event.target, event.currentTarget)) return
    onRowSelect(key)
  }

  function handleRowKeyDown(event: KeyboardEvent<HTMLTableRowElement>, key: string, index: number) {
    if (!onRowSelect) return
    // A key pressed on a control inside a cell is that control's to handle;
    // row selection and arrow navigation stay off until focus is on the
    // row. Table cells hold no interactive content, so this is inert
    // today — revisit the pattern if that changes.
    if (isFromNestedInteractiveElement(event.target, event.currentTarget)) return
    switch (event.key) {
      case 'Enter':
      case ' ':
        event.preventDefault()
        onRowSelect(key)
        break
      case 'ArrowDown':
        event.preventDefault()
        focusRow(index + 1)
        break
      case 'ArrowUp':
        event.preventDefault()
        focusRow(index - 1)
        break
      case 'Home':
        event.preventDefault()
        focusRow(0)
        break
      case 'End':
        event.preventDefault()
        focusRow(rows.length - 1)
        break
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
        {rows.map((row, index) => {
          const key = rowKey(row)
          const selected = selectedRowKeys?.has(key) ?? false
          return (
            <tr
              key={key}
              ref={(node) => {
                rowRefs.current[index] = node
              }}
              aria-selected={onRowSelect ? selected : undefined}
              tabIndex={onRowSelect ? (index === rovingRow ? 0 : -1) : undefined}
              onClick={onRowSelect ? (event) => handleRowClick(event, key) : undefined}
              onKeyDown={onRowSelect ? (event) => handleRowKeyDown(event, key, index) : undefined}
              // Only when the row itself takes focus — not a focusin bubbling
              // up from a future interactive control inside a cell.
              onFocus={
                onRowSelect
                  ? (event) => {
                      if (event.target === event.currentTarget) setActiveRow(index)
                    }
                  : undefined
              }
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
