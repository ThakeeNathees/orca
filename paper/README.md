# Orca Research Papers

## Papers

### [agents-as-code](agents-as-code/) — *Orca: A Declarative Language for AI Agent Orchestration*

The core Orca language paper: declarative DSL design, four-stage compiler pipeline, type system, and editor integration.

### [compiling-intent](compiling-intent/) — *Compiling Intent: An Agentic Compiler for Multi-Agent System Generation*

An agentic compiler (written in Orca itself) that takes natural language prompts and generates `.orca` files, which then compile to Python/LangGraph.

## Prerequisites

- **TeX Live** (or equivalent LaTeX distribution) with `pdflatex`
- **latexmk** — build automation for LaTeX
- **Bibtex** — for bibliography processing

### Installing on Arch Linux

```bash
sudo pacman -S texlive-basic texlive-latex texlive-latexrecommended texlive-latexextra texlive-bibtexextra texlive-fontsrecommended
```

### Installing on Ubuntu/Debian

```bash
sudo apt install texlive-latex-recommended texlive-latex-extra texlive-bibtex-extra latexmk
```

### Installing on macOS

```bash
brew install --cask mactex
```

## Building

```bash
cd agents-as-code && make build
cd compiling-intent && make build
```

Output PDFs are written to `<paper>/out/main.pdf`.
