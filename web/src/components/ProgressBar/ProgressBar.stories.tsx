import type { Meta, StoryObj } from '@storybook/react-vite'
import { ProgressBar } from './ProgressBar'

const meta: Meta<typeof ProgressBar> = {
  title: 'Primitives/ProgressBar',
  component: ProgressBar,
}
export default meta

type Story = StoryObj<typeof ProgressBar>

export const Determinate: Story = { args: { label: 'Import progress', value: 40 } }
export const Indeterminate: Story = { args: { label: 'Loading' } }
