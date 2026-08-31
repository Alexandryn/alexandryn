import type { Meta, StoryObj } from '@storybook/react-vite'
import { MemoryRouter } from 'react-router-dom'
import { WorkGrid } from './WorkGrid'
import type { WorkSummary } from '../../data/library'

const mockWorks: WorkSummary[] = [
  {
    id: 'work-1',
    title: 'Middlemarch',
    subtitle: 'A Study of Provincial Life',
    authors: ['George Eliot'],
    isOwned: true,
    collections: [{ id: 'coll-1', name: 'Victorian Classics' }],
    addedAt: '2026-01-15T10:00:00Z',
  },
  {
    id: 'work-2',
    title: 'Dune',
    subtitle: '',
    authors: ['Frank Herbert'],
    isOwned: true,
    collections: [{ id: 'coll-2', name: 'Sci-Fi' }],
    addedAt: '2026-01-10T12:00:00Z',
  },
  {
    id: 'work-3',
    title: 'Neuromancer',
    subtitle: '',
    authors: ['William Gibson'],
    isOwned: false,
    collections: [],
    addedAt: '2026-01-05T08:00:00Z',
  },
]

const meta: Meta<typeof WorkGrid> = {
  title: 'Components/WorkGrid',
  component: WorkGrid,
  decorators: [
    (Story) => (
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    ),
  ],
}

export default meta
type Story = StoryObj<typeof meta>

export const GridView: Story = {
  args: {
    works: mockWorks,
    view: 'grid',
  },
}

export const ListView: Story = {
  args: {
    works: mockWorks,
    view: 'list',
  },
}
