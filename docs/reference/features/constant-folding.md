# Constant Folding

Orca's analyzer aggressively evaluates expressions at compile time. Anything that can be reduced to a constant — including arithmetic, string concatenation, ternaries, and even lambda calls with constant arguments — is folded before code generation. The generated Python contains the final value, not the computation.

## What gets folded

```orca
let k {
  // Arithmetic, strings, bools — folded to the result
  max_retries = 3 + 2        // -> 5
  greeting    = "hello, " + "world"
  verbose     = true && !false

  // Ternaries with constant conditions collapse to the taken branch
  timeout = (true) ? 30 : 60   // -> 30
}
```

The generated Python for `k.max_retries` is literally `5` — the `+` is gone.

## Lambda calls fold too

If a lambda is pure and its arguments are constants, the call site is replaced with the result:

```orca
let math {
  square = \(x number) -> x * x
  area   = math.square(4)      // -> 16 at compile time
}
```

This extends to higher-order lambdas and currying:

```orca
let math {
  add_k = \(k number) -> \(n number) -> k + n
  add_42 = math.add_k(42)      // -> \(n number) -> 42 + n
  result = math.add_42(8)      // -> 50
}
```

The folded value is inlined wherever it's referenced.

## Why it matters

- **Smaller generated code.** No runtime wrappers around values the compiler already knows.
- **Earlier errors.** Division by zero, out-of-range numbers, or type errors in a folded expression are reported at compile time, pointing at the original source.
- **Predictable codegen.** You can read the generated Python and see exactly the values your Orca source produced.

## Escape hatch

Constant folding only applies when every input is a compile-time constant. Reference an `input` block, a runtime value, or a non-pure expression and the analyzer leaves the computation for runtime.
