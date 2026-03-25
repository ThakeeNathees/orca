# Syntax Overview

<!-- TODO: HCL-like block syntax basics -->
<!-- TODO: Identifiers and naming -->
<!-- TODO: Attributes and assignments -->
<!-- TODO: Types (strings, numbers, booleans, lists) -->
<!-- TODO: References between blocks -->
<!-- TODO: Expressions -->
<!-- TODO: Comments -->

## Strings

All strings in Orca are double-quoted and support both escape sequences and multiple lines. There is no separate "multi-line string" syntax — any string can span lines.

### Escape sequences

| Sequence | Result          |
|----------|-----------------|
| `\n`     | Newline         |
| `\t`     | Tab             |
| `\\`     | Literal `\`     |
| `\"`     | Literal `"`     |

Escape sequences work in all strings, whether single-line or multi-line.

```orca
model gpt4 {
  provider = "openai"
  version  = "gpt-4o\nlatest"
}
```

### Multi-line strings

When a string spans multiple lines, indentation is automatically stripped based on the closing quote's position.

```orca
agent researcher {
  prompt = "
    You are a research assistant.
    You search the web for information.

    Always cite your sources.
    "
}
```

The value of `prompt` is:

```
You are a research assistant.
You search the web for information.

Always cite your sources.
```

#### How indentation stripping works

The **closing quote's column** defines the baseline. That many leading spaces are stripped from every content line.

**Rule:** `baseline = closing_quote_column - 1` (columns are 1-based).

```
  prompt = "
    Hello               ← 4 spaces before content
      Indented          ← 6 spaces (2 extra will remain)
    World               ← 4 spaces before content
    "                   ← closing " at column 5 → baseline = 4
```

Result:

```
Hello
  Indented
World
```

- Lines indented more than the baseline keep their extra indentation.
- Empty lines in the middle are preserved.
- If the first line after the opening `"` is empty (the `"` is followed by a newline), that empty line is removed.
- The last line (whitespace before the closing `"`) is removed.

#### No indentation

If the closing `"` is at column 1, nothing is stripped:

```orca
prompt = "
Hello
World
"
```

Result: `Hello\nWorld`

#### Escape sequences in multi-line strings

Escape sequences work inside multi-line strings too:

```orca
agent a {
  prompt = "
    First line\tsecond column
    Next line
    "
}
```
