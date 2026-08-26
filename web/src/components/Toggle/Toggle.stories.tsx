import type { Meta, StoryObj } from '@storybook/react-vite'
import { Toggle } from './Toggle'

const meta: Meta<typeof Toggle> = {
  title: 'Primitives/Toggle',
  component: Toggle,
}
export default meta

type Story = StoryObj<typeof Toggle>

export const Unchecked: Story = { args: { label: 'Dark mode' } }
export const Checked: Story = { args: { label: 'Dark mode', checked: true } }
export const Disabled: Story = { args: { label: 'Dark mode', disabled: true } }
