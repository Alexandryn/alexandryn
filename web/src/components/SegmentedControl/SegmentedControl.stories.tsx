import type { Meta, StoryObj } from '@storybook/react-vite'
import { SegmentedControl } from './SegmentedControl'

const meta: Meta<typeof SegmentedControl> = {
  title: 'Primitives/SegmentedControl',
  component: SegmentedControl,
}
export default meta

type Story = StoryObj<typeof SegmentedControl>

const options = [
  { value: 'grid', label: 'Grid' },
  { value: 'list', label: 'List' },
  { value: 'table', label: 'Table' },
]

export const Default: Story = { args: { 'aria-label': 'View', options, defaultValue: 'grid' } }
export const Disabled: Story = {
  args: { 'aria-label': 'View', options, defaultValue: 'grid', disabled: true },
}
