---
layout: doc
sidebar: false
aside: false
blog: post
title: Constant Folding (and Why It Matters for Agents)
date: 2026-04-22
category: Language
description: Orca aggressively folds compile-time constants — including lambda calls, recursive functions, and list/map indexing. Bugs that would normally page you at 3 a.m. become red squiggles in your editor.
---

![Folding a recursive lambda call to its result at compile time](/blog-const-folding-cover.png)

Look at the screenshot above. On the left is the Orca source. On the right is the generated Python.

```orca
let vars {
  fib = \(n number) -> (n > 1)
    ? vars.fib(n-1) + vars.fib(n-2)
    : n

  fib_10 = vars.fib(10)
}
```

The thing I want you to notice is the second-to-last line of the Python:

```python
fib_10=55,
```

There is no recursion at runtime. There is no `(lambda n: ...)` being applied to `10`. There is just the number `55`, baked into the generated code. The Orca compiler walked the recursion ten times during semantic analysis and inlined the result.

This is constant folding. It's a normal compiler optimization for languages like C and Rust. What I think is interesting is what it lets you do in a language *for AI agents*.

## What gets folded

Anything whose inputs are all known at compile time. The arithmetic case is the boring one:

```orca
let k {
  max_retries = 3 + 2           // -> 5
  greeting    = "hello, " + "world"
}
```

Ternaries with constant conditions collapse to the taken branch:

```orca
let k {
  timeout = (true) ? 30 : 60   // -> 30
}
```

Lambdas are where it gets interesting. If the lambda is pure and every argument at the call site is a constant, the call disappears and is replaced by the result. This works through closures:

```orca
let math {
  add_k  = \(k number) -> \(n number) -> k + n
  add_42 = math.add_k(42)        // -> \(n number) -> 42 + n
  result = math.add_42(8)        // -> 50
}
```

And it works through recursion, which is what the cover screenshot is showing — `fib(10)` reduces to `55` because every input on the way down is a literal.

## Why I care about this for an agent DSL

A lot of agent code is configuration. You set timeouts, you set max-token caps, you compute prompts from concatenations and conditionals, you index into a list of model names by environment, you pull a key out of a map of provider settings. None of this is the *interesting* part of an agent — but every line of it is something that could be wrong.

The traditional split is: you write Python, the configuration mistakes are caught at runtime, and "runtime" for agents means *after the LLM call has already cost you forty cents and three seconds*.

Constant folding moves the wall. If the value can be known at compile time, the compiler insists on knowing it, and any error in computing it surfaces *as a red squiggle in your editor*. Here's the case that made me build this:

![Index out of range error at compile time](/blog-const-folding-error.png)

`get_index()` returns `10`. The list has three elements. The analyzer folds the call, then folds the index, sees that `10 >= len([1, 2, 3])`, and reports an `index out of range` error pointing exactly at `[vars.get_index()]`. There is no Python generated. There is no agent run. There is no LLM call. The bug is dead at the editor level, before you've even saved.

The same thing happens with map keys (`m[vars.missing_key()]` becomes a compile-time error if `missing_key()` folds to something not in `m`), with division by zero, with out-of-domain numeric ops, and with type mismatches that only become visible *after* a fold reveals the actual constant value.

## What it doesn't fold

Folding only applies when every input is a compile-time constant. The moment you reference a `webhook` payload, the result of a `tool` call, or anything else the compiler can't see, the analyzer leaves the computation alone and emits the equivalent runtime expression.

That's the right boundary. The compiler is conservative about what it knows, and aggressive about what it does with that knowledge.

## How it's implemented (briefly)

The folder is part of the semantic analyzer, not a separate pass. As the analyzer walks the AST and resolves references, it tags each expression with either `(const, value)` or `(non-const, type)`. Whenever it sees a node whose children are all `const`, it evaluates the node right there — arithmetic at the host level, lambda application by substituting the argument constants into the body and recursively folding, list/map indexing by direct lookup. Errors raised during evaluation become source-mapped diagnostics pointing at the original token.

Because folding happens inside the analyzer, type checking sees the *folded* values. A function returning `string | number` whose body folds to `"hello"` is treated as `string` at the call site. This makes the type system precise where it can be precise and conservative where it can't.

If you want to read the actual implementation, it's in [`compiler/analyzer/`](https://github.com/ThakeeNathees/orca/tree/main/compiler/analyzer) — folding lives inside the semantic analyzer alongside reference resolution and type checking.

The point of all of this isn't that constant folding is novel. It's that *for an agent DSL*, moving the wall between "your editor knows" and "your wallet knows" is worth a lot.

— Thakee
