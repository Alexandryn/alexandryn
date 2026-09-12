import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import * as RadixRadioGroup from '@radix-ui/react-radio-group'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

export interface SegmentedControlOption {
  value: string
  label: string
}

export interface SegmentedControlProps extends Omit<
  ComponentPropsWithoutRef<typeof RadixRadioGroup.Root>,
  'aria-label' | 'aria-labelledby'
> {
  options: SegmentedControlOption[]
  'aria-label': string
}

/** Radix RadioGroup, styled as segments — roving-tabindex arrow-key navigation for free. */
export const SegmentedControl = forwardRef<
  ElementRef<typeof RadixRadioGroup.Root>,
  SegmentedControlProps
>(function SegmentedControl({ options, className, ...props }, ref) {
  return (
    <RadixRadioGroup.Root
      ref={ref}
      className={cx('inline-flex rounded-md bg-surface-3 p-4xs gap-4xs', className)}
      {...props}
    >
      {options.map((option) => (
        <RadixRadioGroup.Item
          key={option.value}
          value={option.value}
          className={cx(
            // min-h-11 min-w-11: 44x44 touch-target minimum — py-4xs alone measured 19px tall
            'rounded-sm px-md py-4xs min-h-11 min-w-11 flex items-center justify-center text-xs font-ui text-text-2',
            'data-[state=checked]:bg-surface data-[state=checked]:text-text data-[state=checked]:shadow-sm',
            FOCUS_RING,
            'disabled:opacity-50 disabled:cursor-not-allowed',
          )}
        >
          {option.label}
        </RadixRadioGroup.Item>
      ))}
    </RadixRadioGroup.Root>
  )
})
