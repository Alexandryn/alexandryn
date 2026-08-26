import type { Meta, StoryObj } from '@storybook/react-vite'
import { VisuallyHidden } from './VisuallyHidden'

const meta: Meta<typeof VisuallyHidden> = {
  title: 'Primitives/VisuallyHidden',
  component: VisuallyHidden,
}
export default meta

type Story = StoryObj<typeof VisuallyHidden>

export const IconOnlyButton: Story = {
  render: () => (
    <button className="text-4xl">
      <span aria-hidden="true">×</span>
      <VisuallyHidden>Close dialog</VisuallyHidden>
    </button>
  ),
}
