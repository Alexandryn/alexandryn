import { useEffect, useRef, useState } from 'react'
import { GeneratedCover } from '../../src/components/GeneratedCover/GeneratedCover'

// Benchmark-only fixture (frontend-generated-covers.md FR-4) — a bare
// virtualized container, not the real library-grid layout (phase 06's own
// concern per this spec's Non-goals). Proves GeneratedCover's own render
// cost is cheap enough that virtualization suffices at library scale,
// independent of whatever grid component phase 06 eventually builds.

const ROW_HEIGHT = 220
const COLUMNS = 5
const OVERSCAN_ROWS = 2
const CONTAINER_HEIGHT = 800

export interface CoverGridHarnessProps {
  identifiers: string[]
  onRendered?: () => void
}

export function CoverGridHarness({ identifiers, onRendered }: CoverGridHarnessProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [scrollTop, setScrollTop] = useState(0)

  const rowCount = Math.ceil(identifiers.length / COLUMNS)
  const totalHeight = rowCount * ROW_HEIGHT

  const firstVisibleRow = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN_ROWS)
  const visibleRowCount = Math.ceil(CONTAINER_HEIGHT / ROW_HEIGHT) + OVERSCAN_ROWS * 2
  const lastVisibleRow = Math.min(rowCount, firstVisibleRow + visibleRowCount)

  const visibleItems: { index: number; row: number; col: number }[] = []
  for (let row = firstVisibleRow; row < lastVisibleRow; row++) {
    for (let col = 0; col < COLUMNS; col++) {
      const index = row * COLUMNS + col
      if (index < identifiers.length) visibleItems.push({ index, row, col })
    }
  }

  useEffect(() => {
    onRendered?.()
  })

  return (
    <div
      ref={containerRef}
      data-testid="cover-grid"
      onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
      style={{ height: CONTAINER_HEIGHT, overflowY: 'auto', position: 'relative' }}
    >
      <div style={{ height: totalHeight, position: 'relative' }}>
        {visibleItems.map(({ index, row, col }) => (
          <div
            key={identifiers[index]}
            style={{
              position: 'absolute',
              top: row * ROW_HEIGHT,
              left: `${(col * 100) / COLUMNS}%`,
              width: `${100 / COLUMNS}%`,
              height: ROW_HEIGHT,
              boxSizing: 'border-box',
              padding: 8,
            }}
          >
            <GeneratedCover
              identifier={identifiers[index]!}
              title={`Book ${index}`}
              author={`Author ${index}`}
            />
          </div>
        ))}
      </div>
    </div>
  )
}
