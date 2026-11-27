package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/go-pdf/fpdf"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

//go:embed LXGWWenKai-Regular.ttf
var wenkaiFontBytes []byte

const (
	marginLeft  = 18.0
	marginRight = 18.0
	marginTop   = 20.0
	pageWidth   = 210.0
	contentW    = pageWidth - marginLeft - marginRight
)

type renderContext struct {
	pdf     *fpdf.Fpdf
	source  []byte
	anchors map[string]int
}

func (rc *renderContext) getAnchorID(rawSlug string) int {
	slug := normalizeSlug(rawSlug)
	if id, exists := rc.anchors[slug]; exists {
		return id
	}
	id := rc.pdf.AddLink()
	rc.anchors[slug] = id
	return id
}

// 統一正規化 GitHub 風格錨點（處理 URL 編碼、大小寫、空白轉連字號、移除標點）
func normalizeSlug(s string) string {
	if decoded, err := url.PathUnescape(s); err == nil {
		s = decoded
	}
	s = strings.ToLower(strings.TrimSpace(s))
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else if unicode.IsSpace(r) {
			sb.WriteRune('-')
		}
	}
	return sb.String()
}

type alertStyle struct {
	label         string
	r, g, b       int // 主色（左側色條與標題文字）
	bgR, bgG, bgB int // 淡色背景
}

var githubAlerts = map[string]alertStyle{
	"NOTE":      {label: "【Note】", r: 9, g: 105, b: 218, bgR: 240, bgG: 246, bgB: 255},
	"TIP":       {label: "【Tip】", r: 26, g: 127, b: 55, bgR: 240, bgG: 253, bgB: 244},
	"IMPORTANT": {label: "【Important】", r: 130, g: 80, b: 223, bgR: 248, bgG: 245, bgB: 255},
	"WARNING":   {label: "【Warning】", r: 180, g: 100, b: 0, bgR: 255, bgG: 251, bgB: 235},
	"CAUTION":   {label: "【Caution】", r: 207, g: 34, b: 46, bgR: 254, bgG: 242, bgB: 242},
}

func main() {
	inputFile := flag.String("i", "", "輸入的 Markdown 檔案路徑 (必填)")
	outputFile := flag.String("o", "", "輸出的 PDF 檔案路徑 (選填)")
	flag.Parse()

	if *inputFile == "" {
		log.Fatal("請指定輸入檔案，例如：go run main.go -i README.md")
	}

	outPath := *outputFile
	if outPath == "" {
		ext := filepath.Ext(*inputFile)
		outPath = strings.TrimSuffix(*inputFile, ext) + ".pdf"
	}

	mdBytes, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("讀取檔案失敗: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginLeft, marginTop, marginRight)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddUTF8FontFromBytes("WenKai", "", wenkaiFontBytes)
	pdf.AddUTF8FontFromBytes("WenKai", "B", wenkaiFontBytes)
	pdf.AddUTF8FontFromBytes("WenKai", "I", wenkaiFontBytes)
	pdf.AddUTF8FontFromBytes("WenKai", "U", wenkaiFontBytes)
	if pdf.Error() != nil {
		log.Fatalf("載入字型失敗: %v", pdf.Error())
	}
	pdf.AddPage()
	pdf.SetFont("WenKai", "", 11)

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	reader := text.NewReader(mdBytes)
	doc := md.Parser().Parse(reader)

	rc := &renderContext{
		pdf:     pdf,
		source:  mdBytes,
		anchors: make(map[string]int),
	}

	rc.renderNode(doc, 0)

	if err := pdf.OutputFileAndClose(outPath); err != nil {
		log.Fatalf("輸出 PDF 失敗: %v", err)
	}

	fmt.Printf("成功匯出 PDF：%s\n", outPath)
}

func (rc *renderContext) renderNode(n ast.Node, listDepth int) {
	pdf := rc.pdf
	switch node := n.(type) {
	case *ast.Document:
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			rc.renderNode(child, listDepth)
		}

	case *ast.Heading:
		pdf.Ln(4)
		headingText := extractPlainText(node, rc.source)

		// 同時綁定 Goldmark 自動 ID 與標題純文字生成的 slug，確保各種目錄寫法都能命中
		pdf.SetLink(rc.getAnchorID(headingText), -1, -1)
		if idAttr, ok := node.AttributeString("id"); ok {
			if idBytes, ok := idAttr.([]byte); ok {
				pdf.SetLink(rc.getAnchorID(string(idBytes)), -1, -1)
			}
		}

		sizes := map[int]float64{1: 20, 2: 16, 3: 13.5, 4: 12, 5: 11, 6: 11}
		size := sizes[node.Level]
		pdf.SetFont("WenKai", "B", size)
		pdf.SetTextColor(30, 30, 30)

		pdf.MultiCell(0, size*0.5, headingText, "", "L", false)

		if node.Level <= 2 {
			y := pdf.GetY() + 1
			pdf.SetDrawColor(220, 220, 220)
			pdf.Line(marginLeft, y, pageWidth-marginRight, y)
			pdf.Ln(3)
		} else {
			pdf.Ln(2)
		}
		pdf.SetFont("WenKai", "", 11)

	case *ast.Paragraph:
		rc.renderInline(node, 11, "")
		pdf.Ln(6)

	case *ast.FencedCodeBlock, *ast.CodeBlock:
		pdf.Ln(2)
		pdf.SetFillColor(245, 247, 249)
		pdf.SetDrawColor(225, 228, 232)
		pdf.SetFont("WenKai", "", 9.5)
		pdf.SetTextColor(40, 40, 40)

		var codeLines []string
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			codeLines = append(codeLines, strings.TrimRight(string(line.Value(rc.source)), "\r\n"))
		}
		codeBlock := strings.Join(codeLines, "\n")
		pdf.MultiCell(0, 5.2, codeBlock, "1", "L", true)
		pdf.Ln(4)
		pdf.SetFont("WenKai", "", 11)

	case *ast.Blockquote:
		if style, customTitle, ok := rc.detectAlert(node); ok {
			rc.renderAlertBlock(node, style, customTitle, listDepth)
			return
		}

		// 一般引用區塊維持原本樣式
		pdf.Ln(2)
		startY := pdf.GetY()
		pdf.SetLeftMargin(24)
		pdf.SetTextColor(100, 100, 100)
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			rc.renderNode(child, listDepth)
		}
		endY := pdf.GetY()
		pdf.SetLeftMargin(marginLeft)
		pdf.SetDrawColor(180, 180, 180)
		pdf.SetLineWidth(0.8)
		pdf.Line(20, startY, 20, endY-2)
		pdf.SetLineWidth(0.2)
		pdf.SetTextColor(30, 30, 30)

	case *ast.List:
		baseLeft, _, _, _ := pdf.GetMargins() // 動態取得當前左邊界（支援在 Alert/Blockquote 內縮排）
		idx := node.Start
		if idx == 0 {
			idx = 1
		}
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			indent := baseLeft + float64(listDepth*6)
			pdf.SetLeftMargin(indent)
			pdf.SetX(indent)

			bullet := "• "
			if node.IsOrdered() {
				bullet = fmt.Sprintf("%d. ", idx)
				idx++
			}
			pdf.SetFont("WenKai", "B", 11)
			pdf.Write(6, bullet)
			pdf.SetFont("WenKai", "", 11)

			for itemChild := child.FirstChild(); itemChild != nil; itemChild = itemChild.NextSibling() {
				if _, ok := itemChild.(*ast.List); ok {
					pdf.Ln(5)
					rc.renderNode(itemChild, listDepth+1)
				} else {
					rc.renderInline(itemChild, 11, "")
					pdf.Ln(5.5)
				}
			}
		}
		pdf.SetLeftMargin(baseLeft) // 還原回進入 List 前的左邊界
		pdf.Ln(2)

	case *east.Table:
		rc.renderTable(node)

	case *ast.ThematicBreak:
		pdf.Ln(3)
		y := pdf.GetY()
		pdf.SetDrawColor(210, 210, 210)
		pdf.Line(marginLeft, y, pageWidth-marginRight, y)
		pdf.Ln(4)
	}
}

func (rc *renderContext) renderTable(table *east.Table) {
	pdf := rc.pdf
	pdf.Ln(2)
	type cellData struct {
		text string
		link string
	}
	var rows [][]cellData

	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		var cols []cellData
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			txt := strings.TrimSpace(extractPlainText(cell, rc.source))
			link := extractFirstLink(cell)
			cols = append(cols, cellData{text: txt, link: link})
		}
		rows = append(rows, cols)
	}

	if len(rows) == 0 {
		return
	}

	colCount := len(rows[0])
	colWidth := contentW / float64(colCount)
	paddingX := 2.5
	paddingY := 2.0
	lineHeight := 5.2

	pdf.SetDrawColor(210, 210, 210)

	for rIdx, row := range rows {
		if rIdx == 0 {
			pdf.SetFont("WenKai", "B", 10)
			pdf.SetFillColor(240, 242, 245)
		} else {
			pdf.SetFont("WenKai", "", 10)
			pdf.SetFillColor(255, 255, 255)
		}

		maxLines := 1
		wrappedCols := make([][]string, colCount)
		for cIdx := 0; cIdx < colCount && cIdx < len(row); cIdx++ {
			lines := pdf.SplitText(row[cIdx].text, colWidth-paddingX*2)
			if len(lines) == 0 {
				lines = []string{""}
			}
			wrappedCols[cIdx] = lines
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}

		rowHeight := float64(maxLines)*lineHeight + paddingY*2

		_, pageH := pdf.GetPageSize()
		_, _, _, marginBottom := pdf.GetMargins()
		if pdf.GetY()+rowHeight > pageH-marginBottom {
			pdf.AddPage()
		}

		startX := marginLeft
		startY := pdf.GetY()

		for cIdx := 0; cIdx < colCount; cIdx++ {
			x := startX + float64(cIdx)*colWidth
			pdf.Rect(x, startY, colWidth, rowHeight, "DF")

			if cIdx < len(row) {
				cell := row[cIdx]
				if cell.link != "" {
					pdf.SetTextColor(3, 102, 214)
					if strings.HasPrefix(cell.link, "#") {
						slug := strings.TrimPrefix(cell.link, "#")
						pdf.Link(x, startY, colWidth, rowHeight, rc.getAnchorID(slug))
					} else {
						pdf.LinkString(x, startY, colWidth, rowHeight, cell.link)
					}
				} else {
					pdf.SetTextColor(30, 30, 30)
				}

				for lIdx, lineStr := range wrappedCols[cIdx] {
					pdf.SetXY(x+paddingX, startY+paddingY+float64(lIdx)*lineHeight)
					pdf.CellFormat(colWidth-paddingX*2, lineHeight, lineStr, "", 0, "L", false, 0, "")
				}
			}
		}

		pdf.SetXY(marginLeft, startY+rowHeight)
	}

	pdf.SetTextColor(30, 30, 30)
	pdf.Ln(4)
	pdf.SetFont("WenKai", "", 11)
}

func (rc *renderContext) writeWithLink(txt string, linkURL string) {
	if strings.HasPrefix(linkURL, "#") {
		slug := strings.TrimPrefix(linkURL, "#")
		rc.pdf.WriteLinkID(6, txt, rc.getAnchorID(slug))
	} else if linkURL != "" {
		rc.pdf.WriteLinkString(6, txt, linkURL)
	} else {
		rc.pdf.Write(6, txt)
	}
}

func (rc *renderContext) renderInline(n ast.Node, fontSize float64, linkURL string) {
	pdf := rc.pdf
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch c := child.(type) {
		case *ast.Text:
			txt := string(c.Segment.Value(rc.source))
			rc.writeWithLink(txt, linkURL)
			if c.SoftLineBreak() {
				pdf.Write(6, " ")
			} else if c.HardLineBreak() {
				pdf.Ln(6)
			}

		case *ast.AutoLink:
			rawURL := string(c.URL(rc.source))
			label := string(c.Label(rc.source))
			pdf.SetTextColor(3, 102, 214)
			pdf.SetFont("WenKai", "U", fontSize)
			pdf.WriteLinkString(6, label, rawURL)
			pdf.SetFont("WenKai", "", fontSize)
			pdf.SetTextColor(30, 30, 30)

		case *ast.Emphasis:
			style := "I"
			if c.Level == 2 {
				style = "B"
			}
			pdf.SetFont("WenKai", style, fontSize)
			rc.renderInline(c, fontSize, linkURL)
			pdf.SetFont("WenKai", "", fontSize)

		case *ast.CodeSpan:
			pdf.SetFont("WenKai", "", fontSize-1)
			pdf.SetTextColor(180, 40, 40)
			codeTxt := " " + extractPlainText(c, rc.source) + " "
			rc.writeWithLink(codeTxt, linkURL)
			pdf.SetTextColor(30, 30, 30)
			pdf.SetFont("WenKai", "", fontSize)

		case *ast.Link:
			dest := string(c.Destination)
			pdf.SetTextColor(3, 102, 214)
			pdf.SetFont("WenKai", "U", fontSize)
			rc.renderInline(c, fontSize, dest)
			pdf.SetFont("WenKai", "", fontSize)
			pdf.SetTextColor(30, 30, 30)

		default:
			rc.renderInline(c, fontSize, linkURL)
		}
	}
}

func extractPlainText(n ast.Node, source []byte) string {
	var sb strings.Builder
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			sb.Write(t.Segment.Value(source))
		} else if al, ok := child.(*ast.AutoLink); ok {
			sb.Write(al.Label(source))
		} else {
			sb.WriteString(extractPlainText(child, source))
		}
	}
	return sb.String()
}

func extractFirstLink(n ast.Node) string {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if l, ok := child.(*ast.Link); ok {
			return string(l.Destination)
		}
		if sub := extractFirstLink(child); sub != "" {
			return sub
		}
	}
	return ""
}

// 偵測 Blockquote 是否以 [!NOTE] / [!TIP] / [!IMPORTANT] / [!WARNING] / [!CAUTION] 開頭
func (rc *renderContext) detectAlert(bq *ast.Blockquote) (alertStyle, string, bool) {
	firstPara, ok := bq.FirstChild().(*ast.Paragraph)
	if !ok {
		return alertStyle{}, "", false
	}

	// 只取第一段的純文字，並切出第一行
	fullText := strings.TrimSpace(extractPlainText(firstPara, rc.source))
	firstLine := fullText
	if nlIdx := strings.IndexAny(fullText, "\r\n"); nlIdx != -1 {
		firstLine = strings.TrimSpace(fullText[:nlIdx])
	}
	upperLine := strings.ToUpper(firstLine)

	for key, style := range githubAlerts {
		tag := "[!" + key + "]"
		if strings.HasPrefix(upperLine, tag) {
			// 取出同行後面的自訂標題（例如 "閱讀指引"）
			customTitle := strings.TrimSpace(firstLine[len(tag):])
			return style, customTitle, true
		}
	}
	return alertStyle{}, "", false
}

// 渲染 GitHub 風格的有色 Alert 區塊
func (rc *renderContext) renderAlertBlock(bq *ast.Blockquote, style alertStyle, customTitle string, listDepth int) {
	pdf := rc.pdf
	pdf.Ln(2)

	// 剝除第一段開頭的第一行（即 "[!WARNING] 閱讀指引"）
	firstPara := bq.FirstChild().(*ast.Paragraph)
	for child := firstPara.FirstChild(); child != nil; {
		next := child.NextSibling()
		if t, ok := child.(*ast.Text); ok {
			isBreak := t.SoftLineBreak() || t.HardLineBreak()
			firstPara.RemoveChild(firstPara, child)
			if isBreak {
				break // 刪到第一個換行就停止，保留同一段落後面的文字
			}
		} else {
			firstPara.RemoveChild(firstPara, child)
		}
		child = next
	}

	_, pageH := pdf.GetPageSize()
	_, _, _, marginBottom := pdf.GetMargins()
	if pdf.GetY()+22 > pageH-marginBottom {
		pdf.AddPage()
	}

	startPage := pdf.PageNo()
	startY := pdf.GetY()

	boxX := marginLeft
	boxW := contentW
	innerLeft := marginLeft + 5.0
	innerRight := marginRight + 4.0

	pdf.SetLeftMargin(innerLeft)
	pdf.SetRightMargin(innerRight)
	pdf.SetXY(innerLeft, startY+3)

	// 1. 繪製 Alert 標題（若同行有寫 "閱讀指引" 會自動接在後面，變成 "【Warning】 閱讀指引"）
	title := style.label
	if customTitle != "" {
		title = style.label + " " + customTitle
	}
	pdf.SetFont("WenKai", "B", 11)
	pdf.SetTextColor(style.r, style.g, style.b)
	pdf.Write(6, title)
	pdf.Ln(6.5)

	// 2. 繪製內部段落、清單或程式碼
	pdf.SetFont("WenKai", "", 10.5)
	pdf.SetTextColor(36, 41, 47)
	for child := bq.FirstChild(); child != nil; child = child.NextSibling() {
		if child == firstPara && strings.TrimSpace(extractPlainText(firstPara, rc.source)) == "" {
			continue
		}
		rc.renderNode(child, listDepth)
	}

	endPage := pdf.PageNo()
	endY := pdf.GetY() + 1

	// 還原外層預設邊界
	pdf.SetLeftMargin(marginLeft)
	pdf.SetRightMargin(marginRight)
	pdf.SetFont("WenKai", "", 11)
	pdf.SetTextColor(30, 30, 30)

	// 3. 畫圓角外框與左側 1.5mm 實心粗色條
	if startPage == endPage {
		boxH := endY - startY
		pdf.SetDrawColor(style.r, style.g, style.b)
		pdf.SetLineWidth(0.35)
		pdf.RoundedRect(boxX, startY, boxW, boxH, 1.5, "1234", "D")

		pdf.SetFillColor(style.r, style.g, style.b)
		pdf.Rect(boxX, startY, 1.5, boxH, "F")
	}

	pdf.SetLineWidth(0.2)
	pdf.SetXY(marginLeft, endY+3)
}
