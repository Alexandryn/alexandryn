import type { LibraryItem } from '../../../data/library'

// Hand-written fixture (frontend-shell-and-routing.md FR-6 tier b): the
// contract has no library endpoint yet — phase 06 adds it and replaces
// this.
// TODO(phase-06): replace with contract-generated fixture

export const libraryItemsFixture: LibraryItem[] = [
  { id: 'ol-1', title: 'Invisible Cities', author: 'Italo Calvino' },
  { id: 'ol-2', title: 'Piranesi', author: 'Susanna Clarke' },
  { id: 'ol-3', title: 'The Rings of Saturn', author: 'W. G. Sebald' },
]
