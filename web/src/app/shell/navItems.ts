export interface NavItem {
  to: string
  label: string
  /** A rule above this item in the sidebar (the canvas's divider). */
  dividerBefore?: boolean
}

// Order and grouping from .design-reference/Alexandryn-Electron.dc.html's
// sidebar: Library / Discover / Sources / Collections — rule — Activity /
// Import / Settings. Which of these are host-only is declared once, in
// src/app/routes.tsx's `hostOnly()` wrapper — not duplicated here; every
// link renders while the mock grants all capabilities.
export const NAV_ITEMS: NavItem[] = [
  { to: '/library', label: 'Library' },
  { to: '/discover', label: 'Discover' },
  { to: '/sources', label: 'Sources' },
  { to: '/collections', label: 'Collections' },
  { to: '/activity', label: 'Activity', dividerBefore: true },
  { to: '/import', label: 'Import' },
  { to: '/settings', label: 'Settings' },
]

// The mobile tab bar's four destinations (spec FR-3, and the Mobile
// canvas's tabbar()): library / discover / collections / more.
export const TAB_ITEMS: Pick<NavItem, 'to' | 'label'>[] = [
  { to: '/library', label: 'Library' },
  { to: '/discover', label: 'Discover' },
  { to: '/collections', label: 'Collections' },
  { to: '/more', label: 'More' },
]
