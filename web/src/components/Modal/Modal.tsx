import type { ReactNode } from 'react'
import * as RadixDialog from '@radix-ui/react-dialog'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

export interface ModalProps {
  open?: boolean
  onOpenChange?: (open: boolean) => void
  trigger?: ReactNode
  title: string
  description?: string
  children?: ReactNode
  /** Extra classes for the dialog panel — e.g. a narrower max-width. */
  contentClassName?: string
  /** When false, the corner close button is hidden (Escape/overlay still close). */
  showClose?: boolean
}

/**
 * Radix Dialog (FR-1) — focus trap (FocusScope), focus-return-on-close
 * (onCloseAutoFocus), Escape-to-close, and background inert-marking
 * (aria-hidden via the `aria-hidden` package) all come from Radix, not
 * reimplemented by hand.
 */
export function Modal({
  open,
  onOpenChange,
  trigger,
  title,
  description,
  children,
  contentClassName,
  showClose = true,
}: ModalProps) {
  return (
    <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
      {trigger && <RadixDialog.Trigger asChild>{trigger}</RadixDialog.Trigger>}
      <RadixDialog.Portal>
        <RadixDialog.Overlay
          className={cx(
            'fixed inset-0 bg-scrim/50 transition-opacity',
            'data-[state=closed]:opacity-0 data-[state=open]:opacity-100 motion-reduce:transition-none',
          )}
        />
        <RadixDialog.Content
          className={cx(
            'fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2',
            // max-w-[48rem] not max-w-3xl: this project's --spacing-3xl
            // token collides with Tailwind's max-w-3xl scale key, so the
            // named class silently resolved to 20px (audit 0017 A-17-08).
            'rounded-lg bg-surface p-xl shadow-lg max-w-[48rem] w-full',
            'transition-opacity data-[state=closed]:opacity-0 data-[state=open]:opacity-100 motion-reduce:transition-none',
            contentClassName,
          )}
        >
          <RadixDialog.Title className="text-lg font-ui text-text">{title}</RadixDialog.Title>
          {description && (
            <RadixDialog.Description className="text-xs text-text-2 mt-4xs">
              {description}
            </RadixDialog.Description>
          )}
          {children}
          {showClose && (
            <RadixDialog.Close
              aria-label="Close"
              className={cx('absolute top-md right-md text-text-2', FOCUS_RING)}
            >
              <span aria-hidden="true">×</span>
            </RadixDialog.Close>
          )}
        </RadixDialog.Content>
      </RadixDialog.Portal>
    </RadixDialog.Root>
  )
}
