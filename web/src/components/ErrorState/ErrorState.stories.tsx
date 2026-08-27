import type { Meta, StoryObj } from '@storybook/react-vite'
import { ErrorState } from './ErrorState'

const meta: Meta<typeof ErrorState> = {
  title: 'Primitives/ErrorState',
  component: ErrorState,
}
export default meta

type Story = StoryObj<typeof ErrorState>

export const WithRetryAndReference: Story = {
  args: {
    title: "Couldn't load your library",
    description: 'The connection to the server was interrupted.',
    code: 'unavailable',
    correlationId: '9f2c1a7e-4b0d-4c8a-9e11-2f6b8d3a5c74',
    onRetry: () => {},
  },
}

export const MessageOnly: Story = {
  args: {
    title: 'That collection no longer exists',
  },
}
