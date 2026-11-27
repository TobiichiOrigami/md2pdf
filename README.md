# md2pdf

Convert a Markdown file to PDF (Go, goldmark + fpdf, CJK font LXGW WenKai embedded).

## Requirements

- Go 1.25+
- `LXGWWenKai-Regular.ttf` in project root (embedded at build time via `go:embed`)

## Install dependencies

```bash
go mod download
```

## Run

```bash
go run . -i input.md -o output.pdf
```

- `-i` input Markdown file (required)
- `-o` output PDF file (optional)

## Build

```bash
go build -o md2pdf .
./md2pdf -i input.md -o output.pdf
```
