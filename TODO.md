# Orca Studio — Improvement Plan

Systematic cleanup of the vibe-coded studio UI. Organized into phases so each
phase leaves the codebase in a shippable state.

> **Store note:** The Zustand store is intentionally in-memory for now.
> A future milestone will introduce a **dual-mode store** (browser-only with
> localStorage/IndexedDB vs. backend-synced) — keep that in mind when touching
> state but don't implement persistence yet.

---

## Phase 0 — Tooling & Test Infrastructure

Set up the test framework so every subsequent phase can ship with tests.

- [x] Add **Vitest** + **React Testing Library** + **jsdom**
  - `pnpm add -D vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom`
  - Add `vitest.config.ts` with jsdom environment, path aliases matching `tsconfig.json`
  - Add `"test": "vitest run"` and `"test:watch": "vitest"` scripts to `package.json`
- [x] Add a **smoke test** that renders `<Page />` without crashing (validates the setup works)
- [x] Add ESLint rule `no-restricted-syntax` to ban `div[role="button"]` going forward

---

## Phase 1 — Error Handling & Resilience

Prevent full-app crashes and improve robustness.

- [x] Create a reusable `<ErrorBoundary>` component (`components/error-boundary.tsx`)
  - Render a fallback UI with "Something went wrong" + retry button
  - Log errors to console (prep for future error reporting)
- [x] Wrap **Canvas**, **Inspector**, **Palette**, and **CodeEditor** in error boundaries
  - Each panel should crash independently without taking down the whole app
- [x] Add error boundaries around **dashboard** and **project-sidebar** as well
- [x] **Tests:**
  - Test that ErrorBoundary renders fallback when child throws
  - Test that retry button re-mounts the child

---

## Phase 2 — Deduplicate & Extract Shared Code

Eliminate the most impactful code duplication.

### 2a — Shared icon map

- [x] Extract `ICON_MAP` from `palette.tsx` and `nodes/base-node.tsx` into `lib/icons.ts`
- [x] Both files import from the shared module
- [x] **Test:** Unit test that every `BlockKind` in `block-defs.ts` has a corresponding icon entry

### 2b — Reusable context menu / dropdown

- [x] Create reusable `DropdownMenu` component in `components/ui/dropdown-menu.tsx`
- [x] Replace the hand-rolled menu in **`project-sidebar.tsx`** (getBoundingClientRect + portal)
- [x] Replace the hand-rolled menu in **`dashboard.tsx`** (`WorkflowMenu` component)
- [x] Remove all manual `getBoundingClientRect()` positioning logic

### 2c — Shared input styling

- [x] Extract repeated input class strings into constants in `lib/styles.ts`
- [x] Apply consistently in `inspector.tsx`

---

## Phase 3 — Accessibility & Correctness Fixes

### 3a — Replace `div[role="button"]` with `<button>`

- [x] `project-sidebar.tsx` — `ProjectItem` uses `<div role="button">`, change to `<button>`
- [x] `dashboard.tsx` — any clickable divs should be real buttons
- [x] Ensure keyboard handlers (Enter/Space) work natively via `<button>`

### 3b — Fix unsafe type casting

- [x] `nav-sidebar.tsx` — replace `delete p.type` cast pattern with destructuring
- [x] `palette.tsx` — same pattern, same fix

### 3c — Escape key handling consolidation

- [x] Create a `useEscapeKey(callback)` hook in `lib/hooks/use-escape-key.ts`
- [x] Replace manual `keydown` + `"Escape"` listeners in `dialog.tsx` and `dropdown-menu.tsx`
- [x] **Tests:** 4 tests for useEscapeKey (fires on Escape, ignores other keys, disabled, cleanup)

---

## Phase 4 — Design Token Cleanup

### 4a — Remove hardcoded hex colors

- [x] Audit all `.tsx` files for hardcoded hex values
- [x] Replace with Tailwind classes: `bg-red-500`, `bg-green-300`, `text-zinc-900`, `dark:bg-zinc-900`, etc.
- [x] Export `HANDLE_COLOR_FALLBACK` from `handle-colors.ts`, use in `base-node.tsx`
- [x] Zero hardcoded hex colors remaining in `components/`

### 4b — Extract magic numbers

- [x] Document z-index stacking order in `globals.css` (z-10 handles, z-50 overlays, z-100 dialog)
- [ ] Extract repeated dimensions (sidebar width `52px`, palette width, etc.) into CSS variables or constants

---

## Phase 5 — Component Tests

Now that the code is clean, add meaningful test coverage.

### Store tests (`lib/store.test.ts`) — 15 tests
- [x] `addNode` creates a node with correct defaults
- [x] `removeNode` removes the node AND cascading edges
- [x] `removeNode` clears selectedNodeId if removed node was selected
- [x] `updateNodeData` merges field data correctly
- [x] `updateNodeLabel` updates the label
- [x] `onConnect` creates an edge between nodes
- [x] `createWorkflow` creates and switches to editor
- [x] `deleteWorkflow` removes the workflow
- [x] `createProject` creates and sets active
- [x] `renameProject` renames the project
- [x] `deleteProject` deletes project and its workflows
- [x] `deleteProject` falls back active project
- [x] `deleteProject` does not delete the last project
- [x] `goToDashboard` switches to dashboard view

### Block definitions tests (`lib/block-defs.test.ts`) — 6 tests
- [x] Every `BlockKind` has a corresponding `BlockDef`
- [x] `BLOCK_KINDS` matches `BLOCK_DEFS` keys
- [x] All palette groups reference valid block kinds
- [x] `canConnect()` allows same-type connections
- [x] `canConnect()` allows "any" target type
- [x] `canConnect()` allows trigger → agent
- [x] `canConnect()` rejects mismatched types

### Component render tests
- [x] `<Page />` smoke test (renders without crashing, nav sidebar present)

---

## Phase 6 — Performance

Only if needed — profile first.

- [ ] Wrap `PaletteGroup` in `React.memo` (re-renders on every keystroke during search)
- [ ] Wrap `FieldRenderer` in `React.memo` in `inspector.tsx`
- [ ] Memoize `filteredGroups` in palette with `useMemo`
- [ ] Profile React Flow canvas with 50+ nodes — check for jank
- [ ] Consider virtualizing the workflow list in dashboard if it grows large

---

## Phase 7 — Future Architecture (do not start yet)

Tracked here for context. These require design decisions first.

- [ ] **Dual-mode store** — Abstract store behind an interface so it can be backed by:
  - Browser-only: localStorage or IndexedDB (offline-capable)
  - Backend: REST/gRPC API calls with optimistic updates
  - Strategy pattern or adapter pattern on the store layer
- [ ] **UI ↔ Code sync** — Bidirectional sync between canvas editor and `.oc` code editor
- [ ] **Undo/redo** — Zustand middleware (`zustand/middleware` has `temporal` or use `zundo`)
- [ ] **Collaborative editing** — CRDT or OT if multi-user is ever needed
- [ ] **E2E tests** — Playwright for full user flows (create project → add nodes → connect → build)

---

## Checklist Legend

- [ ] Not started
- [x] Done
- ~~Cancelled~~ (strikethrough if no longer needed)
