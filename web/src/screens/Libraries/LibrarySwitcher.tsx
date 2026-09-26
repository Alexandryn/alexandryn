import { QueryClientContext, useQuery, useQueryClient } from '@tanstack/react-query'
import { useContext, useEffect, useMemo } from 'react'
import { getActiveLibraryId, setActiveLibraryId } from '../../data/auth'
import { switchActiveLibrary } from '../../data/activeLibrary'
import { fetchLibraries, type Library } from '../../data/libraries'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

export function LibrarySwitcher() {
  const client = useContext(QueryClientContext)
  if (!client) {
    return null
  }
  return <LibrarySwitcherInner />
}

function LibrarySwitcherInner() {
  const queryClient = useQueryClient()
  const { data } = useQuery({
    queryKey: ['libraries'],
    queryFn: fetchLibraries,
    staleTime: 60_000,
  })

  const libraries = useMemo(() => data?.libraries || [], [data?.libraries])
  const activeId = getActiveLibraryId() || (libraries[0]?.id ?? '')

  useEffect(() => {
    if (!getActiveLibraryId() && libraries[0]) {
      setActiveLibraryId(libraries[0].id)
    }
  }, [libraries])

  const handleSelect = (e: React.ChangeEvent<HTMLSelectElement>) => {
    switchActiveLibrary(queryClient, e.target.value)
  }

  if (libraries.length <= 1) {
    return null
  }

  return (
    <div className="flex items-center gap-xs text-xs">
      <label htmlFor="library-switcher" className="text-text-3 font-medium sr-only">
        Active Library:
      </label>
      <select
        id="library-switcher"
        value={activeId}
        onChange={handleSelect}
        className={cx(
          'h-9 px-sm py-xs bg-surface-2 hover:bg-surface border border-border rounded-xs text-sm font-medium text-text transition-colors cursor-pointer',
          FOCUS_RING,
        )}
      >
        {libraries.map((l: Library) => (
          <option key={l.id} value={l.id}>
            {l.name}
          </option>
        ))}
      </select>
    </div>
  )
}

