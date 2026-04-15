# Lambdas & Closures

Orca lambdas are first-class values. They capture bindings from their enclosing scope, support higher-order composition, and can recurse through their defining block.

## Syntax

```orca
\(param type, ...) return_type -> body
```

`\` is visual shorthand for λ. The return type is optional — omit it and the analyzer infers from the body. The body is a single expression that may span multiple lines.

```orca
let funcs {
  add    = \(a number, b number) number -> a + b
  double = \(x number) -> x * 2
  greet  = \() -> "hello"
}
```

## Closures

Lambdas close over bindings in their enclosing scope, producing a new value each time the outer binding is evaluated:

```orca
let funcs {
  add_k  = \(k number) -> \(n number) -> k + n
  add_42 = funcs.add_k(42)   // closes over k = 42
  result = funcs.add_42(8)   // -> 50
}
```

The captured `k` is resolved at compile time. When a closure's free variables are all constants, the entire application [folds](./constant-folding) to a literal.

## Recursion

A lambda can call itself through its enclosing `let` block:

```orca
let main {
  fib = \(n number) -> (n > 1)
    ? main.fib(n - 1) + main.fib(n - 2)
    : n
}
```

Recursive lambdas are type-checked against their declared signature. If you omit the return type, the analyzer infers it from the non-recursive branch.

## Types

A lambda has type `callable[P1, P2, ..., R]` — the parameter types followed by the return type. `\(a number, b number) number -> a + b` has type `callable[number, number, number]` and is assignable to any field declared with that type.

## Code generation

Each lambda compiles to a Python `lambda` expression. Captured bindings become default arguments so the closure is self-contained at runtime. Where constant folding applies, the lambda is erased entirely and replaced by its result.
