import { describe, it, expect, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useEscapeKey } from "./use-escape-key";

describe("useEscapeKey", () => {
  it("calls callback when Escape is pressed", () => {
    const callback = vi.fn();
    renderHook(() => useEscapeKey(callback));

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(callback).toHaveBeenCalledOnce();
  });

  it("does not call callback for other keys", () => {
    const callback = vi.fn();
    renderHook(() => useEscapeKey(callback));

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
    expect(callback).not.toHaveBeenCalled();
  });

  it("does not call callback when disabled", () => {
    const callback = vi.fn();
    renderHook(() => useEscapeKey(callback, false));

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(callback).not.toHaveBeenCalled();
  });

  it("cleans up listener on unmount", () => {
    const callback = vi.fn();
    const { unmount } = renderHook(() => useEscapeKey(callback));

    unmount();
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(callback).not.toHaveBeenCalled();
  });
});
