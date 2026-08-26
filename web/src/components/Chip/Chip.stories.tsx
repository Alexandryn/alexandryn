import type { Meta, StoryObj } from '@storybook/react-vite'
import { Chip } from './Chip'

const meta: Meta<typeof Chip> = {
  title: 'Primitives/Chip',
  component: Chip,
}
export default meta

type Story = StoryObj<typeof Chip>

export const Plain: Story = { args: { children: 'Fiction' } }
export const Removable: Story = { args: { children: 'Fiction', onRemove: () => {} } }
export const Disabled: Story = { args: { children: 'Fiction', onRemove: () => {}, disabled: true } }
