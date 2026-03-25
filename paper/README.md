# Orca Research Paper

*"Orca: A Declarative Language for AI Agent Orchestration"*

## Prerequisites

- **TeX Live** (or equivalent LaTeX distribution) with `pdflatex`
- **latexmk** — build automation for LaTeX
- **Bibtex** — for bibliography processing

### Installing on Arch Linux

```bash
sudo pacman -S texlive-basic texlive-latex texlive-latexrecommended texlive-latexextra texlive-bibtexextra texlive-fontsrecommended
```

`latexmk` is included in `texlive-basic`.

### Installing on Ubuntu/Debian

```bash
sudo apt install texlive-latex-recommended texlive-latex-extra texlive-bibtex-extra latexmk
```

### Installing on macOS

```bash
brew install --cask mactex
```

`latexmk` is included with MacTeX.

## Building

```bash
make build
```

Output PDF is written to `out/main.pdf`.

## Watch mode

Auto-rebuilds on file changes:

```bash
make watch
```

## Clean

Remove all generated files:

```bash
make clean
```
