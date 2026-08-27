import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import * as RadixToast from '@radix-ui/react-toast'
import { cx } from '../../lib/cx'

export const ToastViewport = forwardRef<
  ElementRef<typeof RadixToast.Viewport>,
  ComponentPropsWithoutRef<typeof RadixToast.Viewport>
>(function ToastViewport({ className, ...props }, ref) {
  return (
    <RadixToast.Viewport
      ref={ref}
      className={cx('fixed bottom-0 right-0 z-50 flex flex-col gap-xs p-lg', className)}
      {...props}
    />
  )
})

export interface ToastProps extends ComponentPropsWithoutRef<typeof RadixToast.Root> {
  title: string
  description?: string
  actionLabel?: string
  onAction?: () => void
}

/**
 * Radix Toast (FR-1) — auto-dismiss timing plus aria-live region
 * management (role="status", aria-live="assertive" by default) is exactly
 * the hard-to-get-right-by-hand complexity this primitive picked Radix for.
 */
export const Toast = forwardRef<ElementRef<typeof RadixToast.Root>, ToastProps>(function Toast(
  { title, description, actionLabel, onAction, className, ...props },
  ref,
) {
  return (
    <RadixToast.Root
      ref={ref}
      className={cx(
        'rounded-lg border border-border bg-surface shadow-lg p-lg flex flex-col gap-4xs',
        'transition-opacity data-[state=closed]:opacity-0 data-[state=open]:opacity-100 motion-reduce:transition-none',
        className,
      )}
      {...props}
    >
      <RadixToast.Title className="text-sm font-ui text-text">{title}</RadixToast.Title>
      {description && (
        <RadixToast.Description className="text-xs text-text-2">
          {description}
        </RadixToast.Description>
      )}
      <div className="flex items-center gap-sm">
        {actionLabel && onAction && (
          <RadixToast.Action altText={actionLabel} asChild>
            <button
              type="button"
              onClick={onAction}
              className={cx(
                'text-xs text-accent underline',
                'focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
              )}
            >
              {actionLabel}
            </button>
          </RadixToast.Action>
        )}
        <RadixToast.Close
          aria-label="Dismiss"
          className={cx(
            'text-xs text-text-2',
            'focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
          )}
        >
          <span aria-hidden="true">×</span>
        </RadixToast.Close>
      </div>
    </RadixToast.Root>
  )
})
