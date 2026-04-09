// Public entry point for the storage module.
//
// `getStorageAdapter()` resolves the correct backend for the current
// runtime and mode. The selection rules:
//   - SSR / no window.indexedDB  → MemoryStorageAdapter (non-persistent,
//     prevents crashes during Next.js server rendering).
//   - NEXT_PUBLIC_STUDIO_MODE=browser (default) → IndexedDBStorageAdapter.
//   - NEXT_PUBLIC_STUDIO_MODE=hosted → reserved for the future REST
//     adapter; falls back to memory until that lands.
//
// The result is memoised so every call site shares one adapter instance.

import type { StorageAdapter } from "./types";
import { IndexedDBStorageAdapter } from "./indexeddb-adapter";
import { MemoryStorageAdapter } from "./memory-adapter";

export type StudioMode = "browser" | "hosted";

export * from "./types";
export { IndexedDBStorageAdapter } from "./indexeddb-adapter";
export { MemoryStorageAdapter } from "./memory-adapter";

let cached: StorageAdapter | null = null;

/** Reads the configured studio mode from the public env var. */
function readMode(): StudioMode {
  const raw = process.env.NEXT_PUBLIC_STUDIO_MODE;
  return raw === "hosted" ? "hosted" : "browser";
}

/** Returns true when IndexedDB is usable in the current runtime. */
function hasIndexedDB(): boolean {
  return typeof globalThis !== "undefined" && "indexedDB" in globalThis;
}

/**
 * Returns the configured StorageAdapter singleton. Safe to call from
 * both server and client contexts; server calls get an in-memory stub.
 */
export function getStorageAdapter(): StorageAdapter {
  if (cached) return cached;
  const mode = readMode();

  if (mode === "browser" && hasIndexedDB()) {
    cached = new IndexedDBStorageAdapter();
  } else {
    // Hosted mode isn't implemented yet, and SSR has no IndexedDB — in
    // both cases we hand back an ephemeral memory adapter so the app
    // keeps rendering.
    cached = new MemoryStorageAdapter();
  }
  return cached;
}

/** Test-only reset hook. Clears the memoised adapter between test cases. */
export function __resetStorageAdapterForTests(): void {
  cached = null;
}
