import { QueryClientContext, useQuery, useQueryClient } from '@tanstack/react-query'
import { useContext, useEffect, useMemo } from 'react'
import { getActiveLibraryId, setActiveLibraryId } from '../../data/auth'
import { fetchLibraries, type Library } from '../../data/libraries'

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
    const newId = e.target.value
    setActiveLibraryId(newId)
    queryClient.invalidateQueries({ queryKey: ['library'] })
    queryClient.invalidateQueries({ queryKey: ['collections'] })
    queryClient.invalidateQueries({ queryKey: ['readingProgress'] })
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
        className="px-sm py-xs bg-surface border border-border rounded text-text focus:outline-none focus:border-accent cursor-pointer"
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

