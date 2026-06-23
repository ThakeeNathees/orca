.PHONY: build clean watch

MAIN = main
OUTDIR = out

build:
	@mkdir -p $(OUTDIR)
	latexmk -pdf -pdflatex="pdflatex -interaction=nonstopmode" \
		-bibtex -outdir=$(OUTDIR) $(MAIN).tex

watch:
	@mkdir -p $(OUTDIR)
	latexmk -pdf -pvc -pdflatex="pdflatex -interaction=nonstopmode" \
		-outdir=$(OUTDIR) $(MAIN).tex

clean:
	latexmk -outdir=$(OUTDIR) -C $(MAIN).tex
	rm -rf $(OUTDIR)
