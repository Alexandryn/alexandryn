import type { Meta, StoryObj } from '@storybook/react-vite'
import { Slider } from './Slider'

const meta: Meta<typeof Slider> = {
  title: 'Primitives/Slider',
  component: Slider,
}
export default meta

type Story = StoryObj<typeof Slider>

export const Default: Story = { args: { label: 'Font size', defaultValue: [40], min: 0, max: 100 } }
export const Disabled: Story = {
  args: { label: 'Font size', defaultValue: [40], min: 0, max: 100, disabled: true },
}
