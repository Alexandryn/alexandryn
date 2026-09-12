import { Provider as ToastProvider } from '@radix-ui/react-toast'
import type { ReactNode } from 'react'
import { Button } from '../../src/components/Button/Button'
import { Chip } from '../../src/components/Chip/Chip'
import { DataTable, type DataTableColumn } from '../../src/components/DataTable/DataTable'
import { EmptyState } from '../../src/components/EmptyState/EmptyState'
import { ErrorState } from '../../src/components/ErrorState/ErrorState'
import { FormatBadge } from '../../src/components/FormatBadge/FormatBadge'
import { GeneratedCover } from '../../src/components/GeneratedCover/GeneratedCover'
import { Input } from '../../src/components/Input/Input'
import { Modal } from '../../src/components/Modal/Modal'
import { ProgressBar } from '../../src/components/ProgressBar/ProgressBar'
import { SegmentedControl } from '../../src/components/SegmentedControl/SegmentedControl'
import { Skeleton } from '../../src/components/Skeleton/Skeleton'
import { Slider } from '../../src/components/Slider/Slider'
import { Spinner } from '../../src/components/Spinner/Spinner'
import { StatCard } from '../../src/components/StatCard/StatCard'
import { StatusPill } from '../../src/components/StatusPill/StatusPill'
import { Toast, ToastViewport } from '../../src/components/Toast/Toast'
import { Toggle } from '../../src/components/Toggle/Toggle'
import { VisuallyHidden } from '../../src/components/VisuallyHidden/VisuallyHidden'

// Every primitive in representative states on one page, so
// @axe-core/playwright can scan them in a real browser — computed contrast,
// ARIA validity, focus order as rendered — beyond what the per-primitive
// jsdom axe tests (Tier 2) see.

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-labelledby={`s-${title.replace(/\s+/g, '-')}`} style={{ marginBottom: '2rem' }}>
      <h2 id={`s-${title.replace(/\s+/g, '-')}`} className="text-lg font-ui text-text">
        {title}
      </h2>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '1rem', alignItems: 'flex-start' }}>
        {children}
      </div>
    </section>
  )
}

const tableColumns: DataTableColumn<{ id: string; title: string; year: string }>[] = [
  { key: 'title', header: 'Title', render: (r) => r.title, sortable: true },
  { key: 'year', header: 'Year', render: (r) => r.year },
]

export function Gallery() {
  return (
    <main
      className="bg-background text-text font-ui"
      style={{ padding: '2rem', minHeight: '100vh' }}
    >
      <h1 className="text-3xl font-medium">Accessibility gallery</h1>

      <Section title="Actions">
        <Button>Primary</Button>
        <Button variant="secondary">Secondary</Button>
        <Button variant="ghost">Ghost</Button>
        <Button disabled>Disabled</Button>
      </Section>

      <Section title="Form controls">
        <Input label="Search" placeholder="Title, author, ISBN" />
        <Input label="Email" error="Enter a valid address" defaultValue="not-an-email" />
        <Toggle label="Sync reading progress" />
        <SegmentedControl
          aria-label="Layout"
          defaultValue="grid"
          options={[
            { value: 'grid', label: 'Grid' },
            { value: 'list', label: 'List' },
          ]}
        />
        <div style={{ width: 200 }}>
          <Slider label="Font size" defaultValue={[40]} min={0} max={100} step={1} />
        </div>
      </Section>

      <Section title="Status and metadata">
        <StatusPill tone="success">Available</StatusPill>
        <StatusPill tone="warning">Syncing</StatusPill>
        <StatusPill tone="error">Offline</StatusPill>
        <FormatBadge format="EPUB" />
        <FormatBadge format="PDF" />
        <Chip onRemove={() => {}} removeLabel="Remove English filter">
          English
        </Chip>
        <StatCard label="Books" value="1,284" hint="4 sources" />
        <ProgressBar label="Import progress" value={64} />
      </Section>

      <Section title="Loading and empty">
        <Spinner label="Loading your library" />
        <div style={{ width: 160 }}>
          <Skeleton />
        </div>
        <EmptyState
          title="Your library is waiting."
          description="Discover a book, connect a source, or import your collection."
        />
        <ErrorState
          title="Couldn't load your library"
          code="unavailable"
          correlationId="9f2c1a7e-4b0d-4c8a-9e11-2f6b8d3a5c74"
          onRetry={() => {}}
        />
      </Section>

      <Section title="Covers">
        <GeneratedCover identifier="ol-work-1" title="Invisible Cities" author="Italo Calvino" />
        <GeneratedCover identifier="ol-work-2" title="Piranesi" />
        <GeneratedCover identifier="ol-work-3" />
      </Section>

      <Section title="Data table">
        <DataTable
          caption="Recent imports"
          columns={tableColumns}
          rows={[
            { id: '1', title: 'Invisible Cities', year: '1972' },
            { id: '2', title: 'Piranesi', year: '2020' },
          ]}
          rowKey={(r) => r.id}
          sort={{ columnKey: 'title', direction: 'asc' }}
          onRowSelect={() => {}}
        />
      </Section>

      <Section title="Overlays">
        {/* Closed by default — an open Radix dialog inerts (aria-hidden)
            the rest of the page, which would make every other primitive
            here unscannable. The test opens it for its own scan. */}
        <Modal
          title="Delete this collection?"
          description="This can't be undone."
          trigger={<Button variant="secondary">Delete collection…</Button>}
        >
          <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem' }}>
            <Button variant="secondary">Cancel</Button>
            <Button>Delete</Button>
          </div>
        </Modal>
        <ToastProvider>
          <Toast title="Import complete" description="12 books added" open duration={Infinity} />
          <ToastViewport />
        </ToastProvider>
      </Section>

      <Section title="Screen-reader text">
        <button type="button">
          <span aria-hidden="true">×</span>
          <VisuallyHidden>Dismiss notification</VisuallyHidden>
        </button>
      </Section>
    </main>
  )
}
