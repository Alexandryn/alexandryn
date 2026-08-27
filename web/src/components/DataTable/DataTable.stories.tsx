import type { Meta, StoryObj } from '@storybook/react-vite'
import { DataTable, type DataTableColumn } from './DataTable'

interface Book {
  id: string
  title: string
  author: string
}

const books: Book[] = [
  { id: '1', title: 'Dune', author: 'Frank Herbert' },
  { id: '2', title: 'Foundation', author: 'Isaac Asimov' },
  { id: '3', title: 'Neuromancer', author: 'William Gibson' },
]

const columns: DataTableColumn<Book>[] = [
  { key: 'title', header: 'Title', render: (b) => b.title, sortable: true },
  { key: 'author', header: 'Author', render: (b) => b.author, sortable: true },
]

const meta: Meta<typeof DataTable<Book>> = {
  title: 'Primitives/DataTable',
  component: DataTable<Book>,
}
export default meta

type Story = StoryObj<typeof DataTable<Book>>

export const Default: Story = {
  args: { caption: 'Books', columns, rows: books, rowKey: (b) => b.id },
}

export const Sorted: Story = {
  args: {
    caption: 'Books',
    columns,
    rows: books,
    rowKey: (b) => b.id,
    sort: { columnKey: 'title', direction: 'asc' },
  },
}

export const Selectable: Story = {
  args: {
    caption: 'Books',
    columns,
    rows: books,
    rowKey: (b) => b.id,
    selectedRowKeys: new Set(['1']),
    onRowSelect: () => {},
  },
}
