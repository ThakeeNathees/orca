import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { debounce } from "./debounce";

describe("debounce", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("collapses rapid calls into a single trailing invocation with the last args", () => {
    const spy = vi.fn();
    const debounced = debounce(spy, 100);

    debounced(1);
    debounced(2);
    debounced(3);
    expect(spy).not.toHaveBeenCalled();

    vi.advanceTimersByTime(100);
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenLastCalledWith(3);
  });

  it("flush runs the pending call immediately", () => {
    const spy = vi.fn();
    const debounced = debounce(spy, 500);
    debounced("a");
    debounced.flush();
    expect(spy).toHaveBeenCalledWith("a");

    // After flush, the timer is cleared — no second call when time advances.
    vi.advanceTimersByTime(500);
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it("flush is a no-op when nothing is pending", () => {
    const spy = vi.fn();
    const debounced = debounce(spy, 100);
    debounced.flush();
    expect(spy).not.toHaveBeenCalled();
  });

  it("cancel drops the pending call", () => {
    const spy = vi.fn();
    const debounced = debounce(spy, 100);
    debounced("x");
    debounced.cancel();
    vi.advanceTimersByTime(500);
    expect(spy).not.toHaveBeenCalled();
  });
});
