# Getting Started

## Prerequisites

- **Go 1.25+** — The Orca compiler is written in Go.

## Installation

Build from source:

```bash
git clone https://github.com/ThakeeNathees/orca.git
cd orca/compiler
make build
```

This produces a binary at `compiler/bin/orca`. Add it to your PATH or run it directly.

## Your first Orca file

Create a file called `main.orca`:

```orca
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.7
}

agent writer {
  model   = gpt4
  persona = "You are a helpful writer."
}
```

## Build

Run the compiler in the same directory:

```bash
orca build
```

This reads all `.orca` files in the current directory and generates:

```
build/
├── main.py
└── pyproject.toml
```

## Run the generated code

```bash
cd build
uv sync
uv run main.py
```

## Next steps

- Learn about [all block types](/reference/syntax-overview) available in Orca.
- See [examples](/examples/simple-agent) of complete Orca projects.
- Read the [CLI reference](/guide/cli) for all available commands.
