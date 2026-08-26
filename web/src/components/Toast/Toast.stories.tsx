import type { Meta, StoryObj } from '@storybook/react-vite'
import { Toast, ToastProvider, ToastViewport } from './Toast'

const meta: Meta<typeof Toast> = {
  title: 'Primitives/Toast',
  component: Toast,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
        <ToastViewport />
      </ToastProvider>
    ),
  ],
}
export default meta

type Story = StoryObj<typeof Toast>

export const Default: Story = { args: { title: 'Import complete', open: true, duration: Infinity } }
export const WithDescriptionAndAction: Story = {
  args: {
    title: 'Import complete',
    description: '12 books added to your library',
    actionLabel: 'Undo',
    onAction: () => {},
    open: true,
    duration: Infinity,
  },
}
