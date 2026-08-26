import type { Meta, StoryObj } from '@storybook/react-vite'
import { StatCard } from './StatCard'

const meta: Meta<typeof StatCard> = {
  title: 'Primitives/StatCard',
  component: StatCard,
}
export default meta

type Story = StoryObj<typeof StatCard>

export const Default: Story = { args: { label: 'Books read', value: 42, hint: 'This year' } }
