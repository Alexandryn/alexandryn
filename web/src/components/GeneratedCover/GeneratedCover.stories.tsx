import type { Meta, StoryObj } from '@storybook/react-vite'
import { GeneratedCover } from './GeneratedCover'

const meta: Meta<typeof GeneratedCover> = {
  title: 'Generated cover/GeneratedCover',
  component: GeneratedCover,
  decorators: [
    (Story) => (
      <div className="w-40">
        <Story />
      </div>
    ),
  ],
}
export default meta

type Story = StoryObj<typeof GeneratedCover>

export const Full: Story = {
  args: { identifier: 'work-dune', title: 'Dune', author: 'Frank Herbert' },
}

export const TitleOnly: Story = {
  args: { identifier: 'work-foundation', title: 'Foundation' },
}

export const NeitherPresent: Story = {
  args: { identifier: 'work-unknown' },
}

// frontend-generated-covers.md Test strategy: "looks intentional at every
// step" is a design-review surface, not a unit test — this story is that
// surface, showing all 3 ladder steps plus pattern-variant diversity
// side by side.
export const DegradationLadder: Story = {
  render: () => (
    <div className="flex gap-lg">
      <div className="w-36">
        <GeneratedCover identifier="work-1" title="Dune" author="Frank Herbert" />
      </div>
      <div className="w-36">
        <GeneratedCover identifier="work-2" title="Foundation" />
      </div>
      <div className="w-36">
        <GeneratedCover identifier="work-3" />
      </div>
    </div>
  ),
}

export const PatternVariety: Story = {
  render: () => (
    <div className="flex gap-lg">
      {['a', 'b', 'c', 'd', 'e', 'f'].map((id) => (
        <div key={id} className="w-24">
          <GeneratedCover identifier={`work-variety-${id}`} title={`Book ${id}`} author="Author" />
        </div>
      ))}
    </div>
  ),
}
