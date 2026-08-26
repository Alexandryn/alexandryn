import type { Meta, StoryObj } from '@storybook/react-vite'
import { Skeleton } from './Skeleton'

const meta: Meta<typeof Skeleton> = {
  title: 'Primitives/Skeleton',
  component: Skeleton,
}
export default meta

type Story = StoryObj<typeof Skeleton>

export const CoverPlaceholder: Story = {
  args: { className: 'h-3xl w-3xl', label: 'Loading book cover' },
}
