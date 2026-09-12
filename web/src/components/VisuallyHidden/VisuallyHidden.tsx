import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import * as RadixVisuallyHidden from '@radix-ui/react-visually-hidden'

export type VisuallyHiddenProps = ComponentPropsWithoutRef<typeof RadixVisuallyHidden.Root>

/**
 * Screen-reader-only text: present in the accessibility tree, never
 * rendered visually. Wraps Radix's own implementation — its inline
 * style is Radix's internal clip-rect technique, not a token-styling
 * exception, and callers never author styles on this component themselves.
 */
export const VisuallyHidden = forwardRef<
  ElementRef<typeof RadixVisuallyHidden.Root>,
  VisuallyHiddenProps
>(function VisuallyHidden(props, ref) {
  return <RadixVisuallyHidden.Root ref={ref} {...props} />
})
