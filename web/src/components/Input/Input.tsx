import { forwardRef, useId, type InputHTMLAttributes } from 'react'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string
  error?: string
  hint?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { label, error, hint, id, className, ...props },
  ref,
) {
  const autoId = useId()
  const inputId = id ?? autoId
  const errorId = error ? `${inputId}-error` : undefined
  const hintId = hint ? `${inputId}-hint` : undefined
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined

  return (
    <div className="flex flex-col gap-4xs">
      <label htmlFor={inputId} className="text-xs text-text-2 font-ui">
        {label}
      </label>
      <input
        ref={ref}
        id={inputId}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={cx(
          'rounded-sm border px-sm py-2xs text-sm bg-surface text-text',
          'hover:border-text-3',
          FOCUS_RING,
          'disabled:opacity-50 disabled:cursor-not-allowed',
          error ? 'border-error' : 'border-border',
          className,
        )}
        {...props}
      />
      {hint && !error && (
        <p id={hintId} className="text-xs text-text-2">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} role="alert" className="text-xs text-error">
          {error}
        </p>
      )}
    </div>
  )
})
