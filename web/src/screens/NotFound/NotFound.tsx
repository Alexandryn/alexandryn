import { useNavigate } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { AlexMascot } from '../../components/Mascot'

/**
 * The catch-all for an unregistered URL — a real "this page doesn't exist" state composed from
 * primitives, never a blank page or the router's undecorated default.
 */
export function NotFound() {
  const navigate = useNavigate()

  return (
    <div className="mx-auto max-w-[40rem] p-3xl text-center">
      <AlexMascot mood="searching" size="lg" className="mx-auto mb-lg" />
      <h1 className="text-3xl font-medium tracking-1">This page doesn't exist</h1>
      <p className="mt-xs text-lg text-text-2">
        The address may be mistyped, or the page may have moved.
      </p>
      <Button variant="secondary" className="mt-lg" onClick={() => navigate('/library')}>
        Go to your library
      </Button>
    </div>
  )
}
