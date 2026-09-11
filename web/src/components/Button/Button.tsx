import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

export type ButtonVariant = 'primary' | 'secondary' | 'ghost'
export type ButtonSize = 'sm' | 'md'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
}

const BASE = cx(
  'inline-flex items-center justify-center gap-xs rounded-md font-ui transition-colors',
  FOCUS_RING,
  'disabled:opacity-50 disabled:cursor-not-allowed motion-reduce:transition-none',
)

const VARIANT: Record<ButtonVariant, string> = {
  primary: 'bg-accent text-accent-text hover:bg-accent/90',
  secondary: 'bg-surface-2 text-text border border-border hover:bg-surface-3',
  ghost: 'bg-transparent text-accent hover:bg-accent-soft',
}

const SIZE: Record<ButtonSize, string> = {
  // min-h-11: 44px touch-target minimum (audit 0017 A-17-10) — px-sm py-4xs
  // alone measured well under 44px tall at mobile width.
  sm: 'px-sm py-4xs min-h-11 text-xs',
  md: 'px-lg py-2xs text-sm',
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'primary', size = 'md', className, type = 'button', ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      className={cx(BASE, VARIANT[variant], SIZE[size], className)}
      {...props}
    />
  )
})
