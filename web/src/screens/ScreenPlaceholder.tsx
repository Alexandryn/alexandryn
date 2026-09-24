import { useParams } from 'react-router-dom'
import { AlexAvatar } from '../components/Mascot'

export interface ScreenPlaceholderProps {
  title: string
  /** One plain sentence on what this screen will hold. */
  note?: string
}

/**
 * A registered route with no real content yet.
 * Still a complete screen: a single <h1>. Focus management on navigation
 * is the shell's job (AppShell's useContentFocusOnRouteChange), not each
 * screen's.
 */
export function ScreenPlaceholder({ title, note }: ScreenPlaceholderProps) {
  return (
    <div className="p-3xl">
      <AlexAvatar size="md" className="mb-md opacity-75" />
      <h1 className="text-3xl font-medium tracking-1">{title}</h1>
      {note ? <p className="mt-xs text-lg text-text-2">{note}</p> : null}
    </div>
  )
}

/** A placeholder that echoes a route param, so a :id route proves it resolves. */
export function ParamPlaceholder({ title, param }: { title: string; param: string }) {
  const params = useParams()
  return <ScreenPlaceholder title={title} note={`${param}: ${params[param] ?? '—'}`} />
}
