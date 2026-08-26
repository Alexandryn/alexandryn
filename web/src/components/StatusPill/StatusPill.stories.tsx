import type { Meta, StoryObj } from '@storybook/react-vite'
import { StatusPill } from './StatusPill'

const meta: Meta<typeof StatusPill> = {
  title: 'Primitives/StatusPill',
  component: StatusPill,
}
export default meta

type Story = StoryObj<typeof StatusPill>

export const StateMatrix: Story = {
  render: () => (
    <div className="flex gap-xs">
      <StatusPill tone="neutral">Draft</StatusPill>
      <StatusPill tone="success">Available</StatusPill>
      <StatusPill tone="warning">Due soon</StatusPill>
      <StatusPill tone="error">Overdue</StatusPill>
      <StatusPill tone="accent">New</StatusPill>
    </div>
  ),
}
