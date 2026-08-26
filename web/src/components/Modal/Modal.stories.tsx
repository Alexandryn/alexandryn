import type { Meta, StoryObj } from '@storybook/react-vite'
import { Button } from '../Button/Button'
import { Modal } from './Modal'

const meta: Meta<typeof Modal> = {
  title: 'Primitives/Modal',
  component: Modal,
}
export default meta

type Story = StoryObj<typeof Modal>

export const Default: Story = {
  args: {
    trigger: <Button>Edit collection</Button>,
    title: 'Edit collection',
    description: 'Rename or delete this collection.',
    children: <Button>Save</Button>,
  },
}
