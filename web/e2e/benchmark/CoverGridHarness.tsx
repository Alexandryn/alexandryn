import { useEffect, useRef, useState } from 'react'
import { GeneratedCover } from '../../src/components/GeneratedCover/GeneratedCover'

// Benchmark-only fixture — a bare virtualized container, not the real
// library-grid layout. Proves GeneratedCover's own render cost is cheap
// enough that virtualization suffices at library scale.

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

  // idealFirst/idealLast are computed before clamping so the overscan
  // window's size stays constant near the edges — clamping firstVisibleRow
  // up to 0 without also trimming lastVisibleRow would otherwise render
  // extra rows exactly at scrollTop=0, where the benchmark's initial-paint
  // measurement happens.
  const viewportRowCount = Math.ceil(CONTAINER_HEIGHT / ROW_HEIGHT)
  const idealFirstRow = Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN_ROWS
  const idealLastRow = idealFirstRow + viewportRowCount + OVERSCAN_ROWS * 2
  const firstVisibleRow = Math.max(0, idealFirstRow)
  const lastVisibleRow = Math.min(rowCount, idealLastRow)

  const visibleItems: { index: number; row: number; col: number }[] = []
  for (let row = firstVisibleRow; row < lastVisibleRow; row++) {
    for (let col = 0; col < COLUMNS; col++) {
      const index = row * COLUMNS + col
      if (index < identifiers.length) visibleItems.push({ index, row, col })
    }
  }

  // Fires once, at mount, regardless of how many times this effect body
  // itself re-runs (StrictMode's dev double-invoke, or a future re-render
  // triggered by something other than mount) — a bare `useEffect(fn)` with
  // no dependency array re-fires on every render, including every
  // scroll-driven re-render from onScroll below, which would otherwise
  // compete with the exact frame budget the benchmark is measuring.
  const hasFiredRef = useRef(false)
  useEffect(() => {
    if (hasFiredRef.current) return
    hasFiredRef.current = true
    onRendered?.()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

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
            // The generated cover itself is aria-hidden — this label
            // is what makes each grid cell satisfy "never the sole
            // accessible name for its book," the same contract
            // GeneratedCover.a11y.test.tsx proves in isolation.
            aria-label={`Book ${index}`}
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
