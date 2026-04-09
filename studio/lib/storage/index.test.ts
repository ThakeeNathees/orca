import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  getStorageAdapter,
  __resetStorageAdapterForTests,
  MemoryStorageAdapter,
  IndexedDBStorageAdapter,
} from "./index";

describe("getStorageAdapter", () => {
  beforeEach(() => {
    __resetStorageAdapterForTests();
    vi.unstubAllEnvs();
  });

  it("returns a memoised instance across calls", () => {
    const a = getStorageAdapter();
    const b = getStorageAdapter();
    expect(a).toBe(b);
  });

  it("falls back to MemoryStorageAdapter when IndexedDB is unavailable", () => {
    const originalDescriptor = Object.getOwnPropertyDescriptor(
      globalThis,
      "indexedDB"
    );
    // Simulate SSR: no indexedDB on globalThis.
    // We use delete on a copied globalThis to avoid polluting jsdom for
    // later tests — restored in the finally block below.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (globalThis as any).indexedDB;
    try {
      const adapter = getStorageAdapter();
      expect(adapter).toBeInstanceOf(MemoryStorageAdapter);
    } finally {
      if (originalDescriptor) {
        Object.defineProperty(globalThis, "indexedDB", originalDescriptor);
      }
    }
  });

  it("returns IndexedDBStorageAdapter when indexedDB is present", () => {
    // jsdom doesn't provide indexedDB by default — stub it.
    const originalDescriptor = Object.getOwnPropertyDescriptor(
      globalThis,
      "indexedDB"
    );
    Object.defineProperty(globalThis, "indexedDB", {
      value: {} as IDBFactory,
      configurable: true,
      writable: true,
    });
    try {
      const adapter = getStorageAdapter();
      expect(adapter).toBeInstanceOf(IndexedDBStorageAdapter);
    } finally {
      if (originalDescriptor) {
        Object.defineProperty(globalThis, "indexedDB", originalDescriptor);
      } else {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        delete (globalThis as any).indexedDB;
      }
    }
  });
});
