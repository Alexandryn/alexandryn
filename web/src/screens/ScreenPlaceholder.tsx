import { useEffect, useRef } from 'react'
import { useParams } from 'react-router-dom'

export interface ScreenPlaceholderProps {
  title: string
  /** One plain sentence on what this screen will hold (constitution §11). */
  note?: string
}

/**
 * A registered route with no real content yet — phase 06 onward replaces
 * these with real screens inside the same shell (this spec's Non-goals).
 * Still a complete screen: a single <h1>, and it takes focus on mount so
 * a keyboard user landing here after navigation isn't dropped at the top
 * of the document.
 */
export function ScreenPlaceholder({ title, note }: ScreenPlaceholderProps) {
  const headingRef = useRef<HTMLHeadingElement>(null)

  useEffect(() => {
    headingRef.current?.focus()
  }, [])

  return (
    <div className="p-3xl">
      <h1 ref={headingRef} tabIndex={-1} className="text-3xl font-medium tracking-1 outline-none">
        {title}
      </h1>
      {note ? <p className="mt-xs text-lg text-text-2">{note}</p> : null}
    </div>
  )
}

/** A placeholder that echoes a route param, so a :id route proves it resolves. */
export function ParamPlaceholder({ title, param }: { title: string; param: string }) {
  const params = useParams()
  return <ScreenPlaceholder title={title} note={`${param}: ${params[param] ?? '—'}`} />
}
