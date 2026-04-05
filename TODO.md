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

- [ ] `project-sidebar.tsx` — `ProjectItem` uses `<div role="button">`, change to `<button>`
- [ ] `dashboard.tsx` — any clickable divs should be real buttons
- [ ] Ensure keyboard handlers (Enter/Space) work natively via `<button>`
- [ ] **Test:** Keyboard navigation works on interactive elements (Enter activates items)

### 3b — Fix unsafe type casting

- [ ] `nav-sidebar.tsx` lines 24-27 — replace `delete p.type` cast pattern with proper destructuring:
  ```tsx
  // Before (unsafe)
  const p = { ...props } as Record<string, unknown>;
  delete p.type;
  const btnProps = p as React.ButtonHTMLAttributes<HTMLButtonElement>;

  // After (safe)
  const { type, ...btnProps } = props;
  ```
- [ ] `palette.tsx` — same pattern, same fix
- [ ] Grep for any other `delete p.` patterns and fix them

### 3c — Escape key handling consolidation

- [ ] Create a `useEscapeKey(callback)` hook in `lib/hooks/use-escape-key.ts`
- [ ] Replace manual `keydown` + `"Escape"` listeners in `dashboard.tsx`, `project-sidebar.tsx`, and `dialog.tsx`

---

## Phase 4 — Design Token Cleanup

### 4a — Remove hardcoded hex colors

- [ ] Audit all `.tsx` files for hardcoded hex values (`#ef4444`, `#dc2626`, `#18181b`, `#86efac`, `#6ee7b7`, etc.)
- [ ] Map each to a Tailwind theme color or CSS variable defined in `globals.css`
- [ ] Update components to use `text-red-500`, `bg-green-300`, etc. or `var(--color-*)` tokens
- [ ] **No visual regressions** — verify in browser

### 4b — Extract magic numbers

- [ ] Collect z-index values (`9999`, `50`, `100`, `z-[100]`) into named constants or Tailwind config
- [ ] Extract repeated dimensions (sidebar width `52px`, palette width, etc.) into CSS variables or constants
- [ ] Document the z-index stacking order somewhere (comment in `globals.css` or a `lib/z-index.ts`)

---

## Phase 5 — Component Tests

Now that the code is clean, add meaningful test coverage.

### Store tests (`lib/store.test.ts`)
- [ ] `addNode` creates a node with correct defaults
- [ ] `removeNode` removes the node AND cascading edges
- [ ] `updateNodeData` merges field data correctly
- [ ] `updateNodeLabel` updates the label
- [ ] `onConnect` creates an edge between valid handles
- [ ] `createWorkflow` / `deleteWorkflow` CRUD operations
- [ ] `createProject` / `renameProject` / `deleteProject` CRUD operations
- [ ] Deleting active project falls back to "Default"
- [ ] Workflow filtering by active project

### Block definitions tests (`lib/block-defs.test.ts`)
- [ ] Every `BlockKind` has a corresponding `BlockDef`
- [ ] `canConnect()` allows valid connections (e.g. model handle → model handle)
- [ ] `canConnect()` rejects invalid connections (e.g. model handle → tool handle)
- [ ] `canConnect()` handles "any" type connections
- [ ] All palette groups reference valid block kinds

### Connection validation tests
- [ ] Self-connections are rejected
- [ ] Duplicate edges are rejected
- [ ] Type-mismatched connections are rejected
- [ ] Valid connections succeed

### Component render tests
- [ ] `<Canvas />` renders without crashing
- [ ] `<Inspector />` renders fields for a selected node
- [ ] `<Palette />` renders all block groups
- [ ] `<Palette />` search filtering works
- [ ] `<Dashboard />` renders workflow list
- [ ] `<BaseNode />` renders correct icon and label for each block kind

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
