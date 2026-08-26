import type { Meta, StoryObj } from '@storybook/react-vite'
import { Input } from './Input'

const meta: Meta<typeof Input> = {
  title: 'Primitives/Input',
  component: Input,
}
export default meta

type Story = StoryObj<typeof Input>

export const Default: Story = { args: { label: 'Title' } }
export const WithHint: Story = { args: { label: 'Title', hint: 'As it appears on the cover' } }
export const Disabled: Story = { args: { label: 'Title', disabled: true } }
export const ErrorState: Story = { args: { label: 'Title', error: 'Title is required' } }
