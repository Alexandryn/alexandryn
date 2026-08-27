import type { Capability } from '../../data/bootstrap'

export interface NavItem {
  to: string
  label: string
  /** Present on host-only sections (architecture-frontend.md FR-3). */
  capability?: Capability
  /** A rule above this item in the sidebar (the canvas's divider). */
  dividerBefore?: boolean
}

// Order and grouping from .design-reference/Alexandryn-Electron.dc.html's
// sidebar: Library / Discover / Sources / Collections — rule — Activity /
// Import / Settings.
export const NAV_ITEMS: NavItem[] = [
  { to: '/library', label: 'Library' },
  { to: '/discover', label: 'Discover' },
  { to: '/sources', label: 'Sources', capability: 'sources' },
  { to: '/collections', label: 'Collections' },
  { to: '/activity', label: 'Activity', dividerBefore: true },
  { to: '/import', label: 'Import', capability: 'import' },
  { to: '/settings', label: 'Settings', capability: 'settings' },
]

// The mobile tab bar's four destinations (spec FR-3, and the Mobile
// canvas's tabbar()): library / discover / collections / more.
export const TAB_ITEMS: Pick<NavItem, 'to' | 'label'>[] = [
  { to: '/library', label: 'Library' },
  { to: '/discover', label: 'Discover' },
  { to: '/collections', label: 'Collections' },
  { to: '/more', label: 'More' },
]
