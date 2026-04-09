# Orca Studio

Visual editor for Orca workflows. Next.js app with a React Flow canvas,
Monaco code view, and a Zustand store.

## Storage

Studio persists projects and workflows through a `StorageAdapter`
interface (`lib/storage/`). The backend is selected at runtime:

| Environment                | Adapter                      | Persistence              |
| -------------------------- | ---------------------------- | ------------------------ |
| Browser tab (default)      | `IndexedDBStorageAdapter`    | Browser IndexedDB        |
| Next.js SSR pass           | `MemoryStorageAdapter`       | None (throwaway stub)    |
| Vitest / jsdom             | `MemoryStorageAdapter`       | None                     |

Selection rules (see `lib/storage/index.ts`):

1. Reads `NEXT_PUBLIC_STUDIO_MODE` — defaults to `browser` when unset.
2. If mode is `browser` **and** `indexedDB` exists on `globalThis`, uses
   `IndexedDBStorageAdapter`.
3. Otherwise falls back to `MemoryStorageAdapter` so SSR and tests do
   not crash.

**In practice: no env var needed — open the app in a browser and data
is persisted to IndexedDB automatically.** Inspect it via DevTools →
Application → IndexedDB → `orca-studio`.

### Adding a real backend later

The adapter interface is deliberately async so a future HTTP-backed
implementation is a drop-in swap:

1. Implement `StorageAdapter` as e.g. `ApiStorageAdapter` (fetch calls).
2. Branch on `mode === "hosted"` in `getStorageAdapter()`.
3. Set `NEXT_PUBLIC_STUDIO_MODE=hosted`.

No store or component changes required.

## Scripts

```sh
pnpm dev      # next dev
pnpm build    # next build
pnpm test     # vitest run
pnpm lint     # eslint
```
