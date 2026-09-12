# Keyboard map

The project-wide keyboard contract.
The same key does the same thing everywhere; `src/test/keyboardMap.test.tsx`
holds this document to the code, exercising one representative primitive
per category.

## Global keys

| Key             | Does                                                                                                                                                               |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Tab**         | Move focus to the next interactive element, in DOM order.                                                                                                           |
| **Shift + Tab** | Move focus to the previous interactive element, in DOM order.                                                                                                      |
| **Enter**       | Activate the focused control (button, link, toggle).                                                                                                               |
| **Space**       | Activate the focused control. When focus is on a scrollable region that is not itself a control, Space scrolls it — standard browser behaviour, never overridden.  |
| **Escape**      | Close or cancel the topmost open transient surface — a modal, a dropdown, a toast. Consistent across every primitive that has one; never a per-component override. |

There is **no global single-key shortcut** (no `g` then `l`
for "go to library", no `/` to focus search beyond the browser's own
find). Shortcuts are deferred until assistive-technology collision research can be
done with that context — a single-letter hotkey designed without it is a
real risk of clashing with a screen reader's own commands.

## Arrow keys — within a composite control only

Arrow keys move focus or adjust a value **inside** a single composite
control. They are never repurposed for page-level navigation, which would
fight a screen reader's own arrow-key browse mode.

| Control            | Arrow behaviour                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Mechanism                                     |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| `SegmentedControl` | Left/Right (and Up/Down) move the selection between segments; the selected segment is the only tab stop (roving tabindex).                                                                                                                                                                                                                                                                                                                                                             | Radix `RadioGroup`                            |
| `Slider`           | Left/Down decrease, Right/Up increase by one step; Home/End jump to min/max; PageUp/PageDown move by a larger step.                                                                                                                                                                                                                                                                                                                                                                    | Radix `Slider`                                |
| `Toggle`           | No arrow behaviour — it is a single control, toggled with Space/Enter.                                                                                                                                                                                                                                                                                                                                                                                                                 | Radix `Switch`                                |
| `DataTable`        | Up/Down move focus between rows; Home/End jump to the first/last row. Only one row is a tab stop at a time (roving tabindex); a further Tab leaves the table. Enter/Space on the focused row toggles its selection. Sortable column headers are ordinary tab stops before the rows; Enter/Space on one toggles its sort. Row-level, not cell-level — the cells hold no interactive content; cell-level roving is added if that changes. | Hand-rolled (no Radix table primitive exists) |

## Focus order

Focus follows DOM order, which follows visual order
(shell composition: titlebar, then
sidebar, then content pane, top to bottom within each). A `tabindex="-1"`
to make an element a script-focus target or to remove a decorative
element from the sequence is fine; a **positive** `tabindex` that
reorders focus is not, and `check:a11y-tabindex` fails CI on one.

The shell offers a "Skip to content" link as the first tab stop; on a
client-side navigation focus moves into the content region so a keyboard
user lands in the new screen, not back at the top of the document.

## Screen-reader-only text

A visually-hidden label uses the shared `<VisuallyHidden>` primitive (or
Tailwind's `sr-only`), never `display: none` or a hand-rolled clip-rect
(`check:a11y-hidden-text`). Prefer visible text; use `aria-label` only
when visible text would be redundant or absent (an icon-only button); use
`alt` only on `<img>`.
