import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import * as RadixSlider from '@radix-ui/react-slider'
import { cx } from '../../lib/cx'

export interface SliderProps extends ComponentPropsWithoutRef<typeof RadixSlider.Root> {
  label: string
}

/** Radix Slider (FR-1) — pointer/keyboard value state machine, not reimplemented by hand. */
export const Slider = forwardRef<ElementRef<typeof RadixSlider.Root>, SliderProps>(function Slider(
  { label, className, ...props },
  ref,
) {
  return (
    <RadixSlider.Root
      ref={ref}
      className={cx('relative flex items-center w-full h-lg touch-none select-none', className)}
      {...props}
    >
      <RadixSlider.Track className="relative h-4xs w-full grow rounded-4xl bg-surface-3">
        <RadixSlider.Range className="absolute h-full rounded-4xl bg-accent" />
      </RadixSlider.Track>
      <RadixSlider.Thumb
        aria-label={label}
        className={cx(
          'block size-md rounded-4xl bg-surface border border-border shadow-sm',
          'focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
          'data-[disabled]:opacity-50',
        )}
      />
    </RadixSlider.Root>
  )
})
