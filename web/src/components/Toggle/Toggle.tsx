import { forwardRef, useId, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import * as RadixSwitch from '@radix-ui/react-switch'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

export interface ToggleProps extends ComponentPropsWithoutRef<typeof RadixSwitch.Root> {
  label: string
}

/** Radix Switch — real ARIA-state-machine complexity (checked/unchecked, keyboard toggle). */
export const Toggle = forwardRef<ElementRef<typeof RadixSwitch.Root>, ToggleProps>(function Toggle(
  { label, id, className, ...props },
  ref,
) {
  const autoId = useId()
  const switchId = id ?? autoId

  return (
    <div className="inline-flex items-center gap-xs">
      <RadixSwitch.Root
        ref={ref}
        id={switchId}
        className={cx(
          'relative h-lg w-3xl rounded-4xl bg-surface-3 data-[state=checked]:bg-accent',
          FOCUS_RING,
          'disabled:opacity-50 disabled:cursor-not-allowed transition-colors motion-reduce:transition-none',
          className,
        )}
        {...props}
      >
        <RadixSwitch.Thumb
          className={cx(
            'block size-md rounded-4xl bg-surface shadow-sm',
            'translate-x-4xs data-[state=checked]:translate-x-lg',
            'transition-transform motion-reduce:transition-none',
          )}
        />
      </RadixSwitch.Root>
      <label htmlFor={switchId} className="text-sm text-text font-ui">
        {label}
      </label>
    </div>
  )
})
