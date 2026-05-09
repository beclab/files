package main

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"os"
	"sort"
	"time"
)

// OOXML types for generating .docx files without external dependencies.

type wDocument struct {
	XMLName xml.Name `xml:"w:document"`
	Wpc     string   `xml:"xmlns:wpc,attr"`
	Mo      string   `xml:"xmlns:mo,attr"`
	Mc      string   `xml:"xmlns:mc,attr"`
	Mv      string   `xml:"xmlns:mv,attr"`
	O       string   `xml:"xmlns:o,attr"`
	R       string   `xml:"xmlns:r,attr"`
	M       string   `xml:"xmlns:m,attr"`
	V       string   `xml:"xmlns:v,attr"`
	Wp14    string   `xml:"xmlns:wp14,attr"`
	Wp      string   `xml:"xmlns:wp,attr"`
	W10     string   `xml:"xmlns:w10,attr"`
	W       string   `xml:"xmlns:w,attr"`
	W14     string   `xml:"xmlns:w14,attr"`
	Wpg     string   `xml:"xmlns:wpg,attr"`
	Wpi     string   `xml:"xmlns:wpi,attr"`
	Wne     string   `xml:"xmlns:wne,attr"`
	Wps     string   `xml:"xmlns:wps,attr"`
	Body    wBody    `xml:"w:body"`
}

type wBody struct {
	Items []interface{}
}

func (b wBody) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "w:body"}
	e.EncodeToken(start)
	for _, item := range b.Items {
		if err := e.Encode(item); err != nil {
			return err
		}
	}
	e.EncodeToken(start.End())
	return nil
}

type wParagraph struct {
	XMLName xml.Name       `xml:"w:p"`
	PPr     *wParagraphPr  `xml:"w:pPr,omitempty"`
	Runs    []wRun         `xml:"w:r"`
}

type wParagraphPr struct {
	XMLName xml.Name    `xml:"w:pPr"`
	PStyle  *wPStyle    `xml:"w:pStyle,omitempty"`
	Jc      *wJc        `xml:"w:jc,omitempty"`
}

type wPStyle struct {
	XMLName xml.Name `xml:"w:pStyle"`
	Val     string   `xml:"w:val,attr"`
}

type wJc struct {
	XMLName xml.Name `xml:"w:jc"`
	Val     string   `xml:"w:val,attr"`
}

type wRun struct {
	XMLName xml.Name `xml:"w:r"`
	RPr     *wRunPr  `xml:"w:rPr,omitempty"`
	Text    wText    `xml:"w:t"`
}

type wRunPr struct {
	XMLName xml.Name `xml:"w:rPr"`
	Bold    *wBold   `xml:"w:b,omitempty"`
	Sz      *wSz     `xml:"w:sz,omitempty"`
	SzCs    *wSzCs   `xml:"w:szCs,omitempty"`
	RFonts  *wRFonts `xml:"w:rFonts,omitempty"`
	Color   *wColor  `xml:"w:color,omitempty"`
}

type wBold struct {
	XMLName xml.Name `xml:"w:b"`
}

type wSz struct {
	XMLName xml.Name `xml:"w:sz"`
	Val     string   `xml:"w:val,attr"`
}

type wSzCs struct {
	XMLName xml.Name `xml:"w:szCs"`
	Val     string   `xml:"w:val,attr"`
}

type wRFonts struct {
	XMLName  xml.Name `xml:"w:rFonts"`
	Ascii    string   `xml:"w:ascii,attr,omitempty"`
	HAnsi    string   `xml:"w:hAnsi,attr,omitempty"`
	EastAsia string   `xml:"w:eastAsia,attr,omitempty"`
}

type wColor struct {
	XMLName xml.Name `xml:"w:color"`
	Val     string   `xml:"w:val,attr"`
}

type wTable struct {
	XMLName xml.Name   `xml:"w:tbl"`
	TblPr   wTblPr     `xml:"w:tblPr"`
	TblGrid wTblGrid   `xml:"w:tblGrid"`
	Rows    []wTableRow `xml:"w:tr"`
}

type wTblPr struct {
	XMLName  xml.Name     `xml:"w:tblPr"`
	TblStyle *wTblStyle   `xml:"w:tblStyle,omitempty"`
	TblW     wTblW        `xml:"w:tblW"`
	Jc       *wJc         `xml:"w:jc,omitempty"`
	TblBorders *wTblBorders `xml:"w:tblBorders,omitempty"`
}

type wTblStyle struct {
	XMLName xml.Name `xml:"w:tblStyle"`
	Val     string   `xml:"w:val,attr"`
}

type wTblW struct {
	XMLName xml.Name `xml:"w:tblW"`
	W       string   `xml:"w:w,attr"`
	Type    string   `xml:"w:type,attr"`
}

type wTblBorders struct {
	XMLName xml.Name `xml:"w:tblBorders"`
	Top     wBorder  `xml:"w:top"`
	Left    wBorder  `xml:"w:left"`
	Bottom  wBorder  `xml:"w:bottom"`
	Right   wBorder  `xml:"w:right"`
	InsideH wBorder  `xml:"w:insideH"`
	InsideV wBorder  `xml:"w:insideV"`
}

type wBorder struct {
	Val   string `xml:"w:val,attr"`
	Sz    string `xml:"w:sz,attr"`
	Space string `xml:"w:space,attr"`
	Color string `xml:"w:color,attr"`
}

type wTblGrid struct {
	XMLName xml.Name     `xml:"w:tblGrid"`
	Cols    []wGridCol   `xml:"w:gridCol"`
}

type wGridCol struct {
	XMLName xml.Name `xml:"w:gridCol"`
	W       string   `xml:"w:w,attr"`
}

type wTableRow struct {
	XMLName xml.Name     `xml:"w:tr"`
	Cells   []wTableCell `xml:"w:tc"`
}

type wTableCell struct {
	XMLName xml.Name     `xml:"w:tc"`
	TcPr    *wTcPr       `xml:"w:tcPr,omitempty"`
	P       []wParagraph `xml:"w:p"`
}

type wTcPr struct {
	XMLName xml.Name `xml:"w:tcPr"`
	Shd     *wShd    `xml:"w:shd,omitempty"`
}

type wShd struct {
	XMLName xml.Name `xml:"w:shd"`
	Val     string   `xml:"w:val,attr"`
	Color   string   `xml:"w:color,attr"`
	Fill    string   `xml:"w:fill,attr"`
}

func newDoc() *wDocument {
	return &wDocument{
		Wpc:  "http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas",
		Mo:   "http://schemas.microsoft.com/office/mac/office/2008/main",
		Mc:   "http://schemas.openxmlformats.org/markup-compatibility/2006",
		Mv:   "urn:schemas-microsoft-com:mac:vml",
		O:    "urn:schemas-microsoft-com:office:office",
		R:    "http://schemas.openxmlformats.org/officeDocument/2006/relationships",
		M:    "http://schemas.openxmlformats.org/officeDocument/2006/math",
		V:    "urn:schemas-microsoft-com:vml",
		Wp14: "http://schemas.microsoft.com/office/word/2010/wordprocessingDrawing",
		Wp:   "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing",
		W10:  "urn:schemas-microsoft-com:office:word",
		W:    "http://schemas.openxmlformats.org/wordprocessingml/2006/main",
		W14:  "http://schemas.microsoft.com/office/word/2010/wordml",
		Wpg:  "http://schemas.microsoft.com/office/word/2010/wordprocessingGroup",
		Wpi:  "http://schemas.microsoft.com/office/word/2010/wordprocessingInk",
		Wne:  "http://schemas.microsoft.com/office/word/2006/wordml",
		Wps:  "http://schemas.microsoft.com/office/word/2010/wordprocessingShape",
	}
}

func mkRun(text string, bold bool, szHalfPt int) wRun {
	sz := fmt.Sprintf("%d", szHalfPt)
	r := wRun{
		Text: wText{Space: "preserve", Content: text},
		RPr: &wRunPr{
			RFonts: &wRFonts{Ascii: "Microsoft YaHei", HAnsi: "Microsoft YaHei", EastAsia: "Microsoft YaHei"},
			Sz:     &wSz{Val: sz},
			SzCs:   &wSzCs{Val: sz},
		},
	}
	if bold {
		r.RPr.Bold = &wBold{}
	}
	return r
}

func mkRunColor(text string, bold bool, szHalfPt int, color string) wRun {
	r := mkRun(text, bold, szHalfPt)
	r.RPr.Color = &wColor{Val: color}
	return r
}

type wText struct {
	XMLName xml.Name `xml:"w:t"`
	Space   string   `xml:"xml:space,attr"`
	Content string   `xml:",chardata"`
}

func heading(text string, level int) wParagraph {
	return wParagraph{
		PPr: &wParagraphPr{PStyle: &wPStyle{Val: fmt.Sprintf("Heading%d", level)}},
		Runs: []wRun{mkRun(text, true, 28)},
	}
}

func para(text string, sz int) wParagraph {
	return wParagraph{Runs: []wRun{mkRun(text, false, sz)}}
}

func labelValue(label, value string) wParagraph {
	return wParagraph{
		Runs: []wRun{
			mkRun(label+"：", true, 20),
			mkRun(value, false, 20),
		},
	}
}

func thinBorders() *wTblBorders {
	b := wBorder{Val: "single", Sz: "4", Space: "0", Color: "999999"}
	return &wTblBorders{Top: b, Left: b, Bottom: b, Right: b, InsideH: b, InsideV: b}
}

func headerCell(text string) wTableCell {
	return wTableCell{
		TcPr: &wTcPr{Shd: &wShd{Val: "clear", Color: "auto", Fill: "1F4E79"}},
		P: []wParagraph{{
			PPr: &wParagraphPr{Jc: &wJc{Val: "center"}},
			Runs: []wRun{mkRunColor(text, true, 16, "FFFFFF")},
		}},
	}
}

func dataCell(text string, align string) wTableCell {
	return wTableCell{
		P: []wParagraph{{
			PPr: &wParagraphPr{Jc: &wJc{Val: align}},
			Runs: []wRun{mkRun(text, false, 14)},
		}},
	}
}

func writeDocx(results []BenchResult, path string, cfg Config) {
	doc := newDoc()
	var items []interface{}

	// Title
	titleP := wParagraph{
		PPr: &wParagraphPr{
			PStyle: &wPStyle{Val: "Title"},
			Jc:     &wJc{Val: "center"},
		},
		Runs: []wRun{mkRun("Files 服务 API 响应时间基准测试报告", true, 36)},
	}
	items = append(items, titleP)

	// Test conditions
	authMode := "X-Bfl-User（内部）"
	if cfg.Cookie != "" {
		authMode = "Cookie（外部链路）"
	}
	proto := "HTTP"
	if len(cfg.BaseURL) > 5 && cfg.BaseURL[:5] == "https" {
		proto = "HTTPS"
	}
	bigDir := "否"
	if cfg.BigDir {
		bigDir = "是"
	}
	conditions := []struct{ label, value string }{
		{"目标地址", cfg.BaseURL},
		{"测试用户", cfg.Owner},
		{"采样次数", fmt.Sprintf("每接口 %d 次", cfg.Samples)},
		{"并发数", fmt.Sprintf("%d", cfg.Concurrency)},
		{"超时设置", cfg.Timeout.String()},
		{"上传分片大小", formatUploadSizes(cfg.UploadSizes)},
		{"大目录测试", bigDir},
		{"鉴权方式", authMode},
		{"协议", proto},
		{"生成时间", time.Now().Format("2006-01-02 15:04:05")},
	}
	for _, c := range conditions {
		items = append(items, labelValue(c.label, c.value))
	}

	// Summary
	items = append(items, heading("一、测试概要", 1))

	var okN, erroredN, setupN, cleanupN, skippedN int
	for _, r := range results {
		switch {
		case r.Note == "skip" || r.Note == "skip-dep":
			skippedN++
		case r.Note == "setup":
			setupN++
		case r.Note == "cleanup":
			cleanupN++
		case r.Status <= 0:
			erroredN++
		default:
			okN++
		}
	}
	summaryData := [][]string{
		{"指标", "数量"},
		{"接口总数", fmt.Sprintf("%d", len(results))},
		{"已测试", fmt.Sprintf("%d", okN)},
		{"前置准备", fmt.Sprintf("%d", setupN)},
		{"清理操作", fmt.Sprintf("%d", cleanupN)},
		{"跳过（不安全）", fmt.Sprintf("%d", skippedN)},
		{"错误", fmt.Sprintf("%d", erroredN)},
	}
	sumTbl := buildSimpleTable(summaryData, 2, true)
	items = append(items, sumTbl)

	// Per-category tables
	items = append(items, heading("二、各分类接口响应时间", 1))
	grouped := groupByCategory(results)
	cats := sortedCategories(grouped)

	for _, cat := range cats {
		group := grouped[cat]
		items = append(items, heading(zhCat(cat)+"（"+cat+"）", 2))

		headers := []string{"方法", "路径", "说明", "请求大小", "平均", "P50", "P95", "首字节", "最小", "最大", "状态码", "当前超时", "备注"}
		var tableData [][]string
		tableData = append(tableData, headers)

		for _, r := range group {
			if r.Note == "skip" || r.Note == "skip-dep" {
				reason := "跳过"
				if r.Note == "skip-dep" {
					reason = "依赖跳过"
				}
				row := []string{r.Route.Method, truncate(r.Route.Pattern, 40), zhDesc(r.Route.Description),
					"-", "-", "-", "-", "-", "-", "-", "跳过", r.Route.CurrentTimeout, reason}
				tableData = append(tableData, row)
				continue
			}
			if r.Status <= 0 && r.Note == "" {
				errMsg := "请求失败"
				if len(r.Samples) > 0 && r.Samples[0].Error != "" {
					errMsg = truncate(r.Samples[0].Error, 30)
				}
				row := []string{r.Route.Method, truncate(r.Route.Pattern, 40), zhDesc(r.Route.Description),
					"-", "-", "-", "-", "-", "-", "-", "错误", r.Route.CurrentTimeout, errMsg}
				tableData = append(tableData, row)
				continue
			}

			note := zhNote(r.Note, r.Status)
			reqSize := avgReqSize(r.Samples)
			avgTTFB := avgTTFBDuration(r.Samples)

			row := []string{
				r.Route.Method,
				truncate(r.Route.Pattern, 40),
				truncate(zhDesc(r.Route.Description), 35),
				fmtBytes(reqSize),
				fmtDuration(r.Avg), fmtDuration(r.P50), fmtDuration(r.P95),
				fmtDuration(avgTTFB),
				fmtDuration(r.Min), fmtDuration(r.Max),
				fmt.Sprintf("%d", r.Status),
				r.Route.CurrentTimeout,
				note,
			}
			tableData = append(tableData, row)
		}

		tbl := buildSimpleTable(tableData, len(headers), true)
		items = append(items, tbl)
	}

	// Timeout recommendations
	items = append(items, heading("三、超时建议", 1))
	items = append(items, para("基于 P95 延迟乘以安全系数（普通接口 3 倍、最低 5 秒，流式接口 5 倍、最低 10 秒），兼顾网络波动与生产环境实际经验。", 18))

	var benchedP95s []time.Duration
	for _, r := range results {
		if r.Note == "skip" || r.Note == "skip-dep" || r.Note == "setup" || r.Note == "cleanup" {
			continue
		}
		if r.Status > 0 && r.P95 > 0 {
			benchedP95s = append(benchedP95s, r.P95)
		}
	}
	sort.Slice(benchedP95s, func(i, j int) bool { return benchedP95s[i] < benchedP95s[j] })

	timeoutHeaders := []string{"分类", "方法", "路径", "P95", "当前超时", "建议超时", "依据"}
	var timeoutData [][]string
	timeoutData = append(timeoutData, timeoutHeaders)

	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note == "setup" || r.Note == "cleanup" || r.Note == "skip" || r.Note == "skip-dep" {
				continue
			}
			if r.Status <= 0 {
				continue
			}
			suggested := suggestTimeout(r.P95, r.Route.Stream)
			timeoutData = append(timeoutData, []string{
				zhCat(cat), r.Route.Method, truncate(r.Route.Pattern, 40),
				fmtDuration(r.P95), r.Route.CurrentTimeout, fmtDuration(suggested), "实测",
			})
		}
	}
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note != "skip" && r.Note != "skip-dep" {
				continue
			}
			estimated := estimateSkippedTimeout(r, benchedP95s)
			timeoutData = append(timeoutData, []string{
				zhCat(cat), r.Route.Method, truncate(r.Route.Pattern, 40),
				"-", r.Route.CurrentTimeout, fmtDuration(estimated), "估算",
			})
		}
	}

	if len(timeoutData) > 1 {
		tbl := buildSimpleTable(timeoutData, len(timeoutHeaders), true)
		items = append(items, tbl)
	}

	// Skipped routes: summary table + detailed analysis
	var skippedRoutes []BenchResult
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note == "skip" || r.Note == "skip-dep" {
				skippedRoutes = append(skippedRoutes, r)
			}
		}
	}

	if len(skippedRoutes) > 0 {
		items = append(items, heading("四、跳过的接口一览", 1))
		items = append(items, para(fmt.Sprintf("共 %d 个接口被跳过，未执行实际测试。", len(skippedRoutes)), 18))

		skipHeaders := []string{"分类", "方法", "路径", "说明", "当前超时", "建议超时", "跳过原因"}
		var skipTableData [][]string
		skipTableData = append(skipTableData, skipHeaders)
		for _, r := range skippedRoutes {
			reason := zhSkipReason(r.Route.SkipReason)
			if r.Note == "skip-dep" {
				reason = "前置条件不满足"
			}
			suggested := fmtDuration(estimateSkippedTimeout(r, benchedP95s))
			skipTableData = append(skipTableData, []string{
				zhCat(r.Route.Category),
				r.Route.Method,
				truncate(r.Route.Pattern, 40),
				truncate(zhDesc(r.Route.Description), 30),
				r.Route.CurrentTimeout,
				suggested,
				truncate(reason, 40),
			})
		}
		skipTbl := buildSimpleTable(skipTableData, len(skipHeaders), true)
		items = append(items, skipTbl)

		// Detailed analysis for routes that have recommendations
		hasAnalysis := false
		for _, r := range skippedRoutes {
			if r.Route.Recommendation != "" {
				hasAnalysis = true
				break
			}
		}
		if hasAnalysis {
			items = append(items, heading("五、跳过的接口详细分析与建议", 1))
			items = append(items, para("以下接口因安全原因未执行，已基于代码分析给出评估：", 18))
			for _, r := range skippedRoutes {
				if r.Route.Recommendation == "" {
					continue
				}
				items = append(items, heading(r.Route.Method+" "+r.Route.Pattern+"（"+zhCat(r.Route.Category)+"）", 3))
				items = append(items, wParagraph{
					Runs: []wRun{
						mkRun("跳过原因：", true, 18),
						mkRun(zhSkipReason(r.Route.SkipReason), false, 18),
					},
				})
				items = append(items, wParagraph{
					Runs: []wRun{
						mkRun("当前 Nginx 超时：", true, 18),
						mkRun(r.Route.CurrentTimeout, false, 18),
					},
				})
				items = append(items, para("分析建议："+zhRecommendation(r.Route.Recommendation), 18))
			}
		}
	}

	doc.Body.Items = items

	if err := writeDocxFile(doc, path); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write docx: %v\n", err)
	}
}

func buildSimpleTable(data [][]string, cols int, headerShading bool) wTable {
	var gridCols []wGridCol
	colW := 9000 / cols
	for i := 0; i < cols; i++ {
		gridCols = append(gridCols, wGridCol{W: fmt.Sprintf("%d", colW)})
	}

	tbl := wTable{
		TblPr: wTblPr{
			TblW:       wTblW{W: "9000", Type: "dxa"},
			Jc:         &wJc{Val: "center"},
			TblBorders: thinBorders(),
		},
		TblGrid: wTblGrid{Cols: gridCols},
	}

	for i, row := range data {
		tr := wTableRow{}
		for j := 0; j < cols; j++ {
			val := ""
			if j < len(row) {
				val = row[j]
			}
			if i == 0 && headerShading {
				tr.Cells = append(tr.Cells, headerCell(val))
			} else {
				align := "center"
				if j == 1 || j == 2 {
					align = "left"
				}
				tr.Cells = append(tr.Cells, dataCell(val, align))
			}
		}
		tbl.Rows = append(tbl.Rows, tr)
	}

	return tbl
}

func writeDocxFile(doc *wDocument, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// [Content_Types].xml
	ct, _ := zw.Create("[Content_Types].xml")
	ct.Write([]byte(contentTypesXML))

	// _rels/.rels
	rels, _ := zw.Create("_rels/.rels")
	rels.Write([]byte(relsXML))

	// word/_rels/document.xml.rels
	wrels, _ := zw.Create("word/_rels/document.xml.rels")
	wrels.Write([]byte(wordRelsXML))

	// word/styles.xml
	styles, _ := zw.Create("word/styles.xml")
	styles.Write([]byte(stylesXML))

	// word/document.xml
	wdoc, _ := zw.Create("word/document.xml")
	wdoc.Write([]byte(xml.Header))
	enc := xml.NewEncoder(wdoc)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encode document.xml: %w", err)
	}

	return nil
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

const relsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const wordRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="Microsoft YaHei" w:hAnsi="Microsoft YaHei" w:eastAsia="Microsoft YaHei"/>
        <w:sz w:val="20"/>
        <w:szCs w:val="20"/>
      </w:rPr>
    </w:rPrDefault>
  </w:docDefaults>
  <w:style w:type="paragraph" w:styleId="Title">
    <w:name w:val="Title"/>
    <w:pPr><w:jc w:val="center"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="36"/><w:szCs w:val="36"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="28"/><w:szCs w:val="28"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:pPr><w:spacing w:before="200" w:after="80"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="heading 3"/>
    <w:pPr><w:spacing w:before="160" w:after="60"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr>
  </w:style>
</w:styles>`
