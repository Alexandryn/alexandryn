import type { Meta, StoryObj } from '@storybook/react-vite'
import { FormatBadge } from './FormatBadge'

const meta: Meta<typeof FormatBadge> = {
  title: 'Primitives/FormatBadge',
  component: FormatBadge,
}
export default meta

type Story = StoryObj<typeof FormatBadge>

export const StateMatrix: Story = {
  render: () => (
    <div className="flex gap-xs">
      <FormatBadge format="EPUB" />
      <FormatBadge format="PDF" />
      <FormatBadge format="MOBI" />
      <FormatBadge format="CBZ" />
    </div>
  ),
}
