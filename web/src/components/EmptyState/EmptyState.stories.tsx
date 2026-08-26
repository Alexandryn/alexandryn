import type { Meta, StoryObj } from '@storybook/react-vite'
import { EmptyState } from './EmptyState'

const meta: Meta<typeof EmptyState> = {
  title: 'Primitives/EmptyState',
  component: EmptyState,
}
export default meta

type Story = StoryObj<typeof EmptyState>

export const Default: Story = {
  args: {
    title: 'No books yet',
    description: 'Add a source to start building your library.',
    action: { label: 'Add source', onClick: () => {} },
  },
}
