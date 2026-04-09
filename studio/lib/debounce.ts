// Minimal trailing-edge debounce with flush/cancel.
//
// The studio store uses this to collapse rapid graph mutations (node
// drags, field keystrokes) into a single IndexedDB write. `flush` is
// called explicitly before the store switches workflows so in-flight
// edits for the outgoing workflow are never lost nor misattributed.

export interface DebouncedFn<A extends unknown[]> {
  (...args: A): void;
  /** Runs any pending invocation immediately and clears the timer. */
  flush: () => void;
  /** Drops any pending invocation without running it. */
  cancel: () => void;
}

export function debounce<A extends unknown[]>(
  fn: (...args: A) => void,
  ms: number
): DebouncedFn<A> {
  let timer: ReturnType<typeof setTimeout> | null = null;
  let lastArgs: A | null = null;

  const run = () => {
    timer = null;
    if (lastArgs) {
      const args = lastArgs;
      lastArgs = null;
      fn(...args);
    }
  };

  const debounced = ((...args: A) => {
    lastArgs = args;
    if (timer) clearTimeout(timer);
    timer = setTimeout(run, ms);
  }) as DebouncedFn<A>;

  debounced.flush = () => {
    if (timer) {
      clearTimeout(timer);
      run();
    }
  };

  debounced.cancel = () => {
    if (timer) clearTimeout(timer);
    timer = null;
    lastArgs = null;
  };

  return debounced;
}
