package app

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestDecodeXLSXCommentsMapsCommentsToSheetCells(t *testing.T) {
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	writeTestZipFile(t, writer, "xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="AT. 1" sheetId="1" r:id="rId1"/></sheets>
</workbook>`)
	writeTestZipFile(t, writer, "xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`)
	writeTestZipFile(t, writer, "xl/worksheets/sheet1.xml", `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`)
	writeTestZipFile(t, writer, "xl/worksheets/_rels/sheet1.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="comments" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="../comments1.xml"/>
</Relationships>`)
	writeTestZipFile(t, writer, "xl/comments1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <authors><author>Professor</author></authors>
  <commentList>
    <comment ref="B3" authorId="0"><text><r><t>feedback exato</t></r></text></comment>
  </commentList>
</comments>`)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	comments, err := decodeXLSXComments(body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	sheetComments := comments["AT. 1"]
	if len(sheetComments) != 1 {
		t.Fatalf("sheet comments length = %d, want 1", len(sheetComments))
	}
	comment := sheetComments[0]
	if comment.RowIndex != 2 || comment.ColumnIndex != 1 {
		t.Fatalf("cell = (%d, %d), want (2, 1)", comment.RowIndex, comment.ColumnIndex)
	}
	if comment.Text != "feedback exato" || comment.Author != "Professor" {
		t.Fatalf("comment = %#v, want text and author", comment)
	}
}

func TestCleanXLSXThreadedCommentExportText(t *testing.T) {
	raw := `[Threaded comment]
 Your version of Excel allows you to read this threaded comment.
Comment:
	Feedback final`

	if got := cleanXLSXCommentText(raw); got != "Feedback final" {
		t.Fatalf("clean text = %q, want Feedback final", got)
	}
	if got := cleanXLSXCommentAuthor("tc={3cd2dce4-02ae-4d00-9e27-7de665291188}"); got != "" {
		t.Fatalf("clean author = %q, want empty", got)
	}
}

func TestWorkbookCommentsAreFilteredToConfiguredGradeSheets(t *testing.T) {
	input := map[string][]workbookCellComment{
		"AT. 1": {
			{SheetName: "AT. 1", Text: "feedback atividade"},
		},
		"Projeto AB2": {
			{SheetName: "Projeto AB2", Text: "feedback projeto"},
		},
		"Base de dados": {
			{SheetName: "Base de dados", Text: "comentario de login"},
		},
		"Controle interno": {
			{SheetName: "Controle interno", Text: "comentario interno"},
		},
	}
	allowed := configuredGradeSheetSet(Config{
		LoginSheet: "Base de dados",
		AB1Tables:  []TableConfig{{SheetName: "AT. 1"}},
		AB2Tables:  []TableConfig{{SheetName: "Projeto AB2"}},
	})

	filtered := filterWorkbookComments(input, allowed)
	if len(filtered) != 2 {
		t.Fatalf("filtered sheets = %d, want 2: %#v", len(filtered), filtered)
	}
	if len(filtered["AT. 1"]) != 1 || len(filtered["Projeto AB2"]) != 1 {
		t.Fatalf("expected configured grade sheets in %#v", filtered)
	}
	if _, ok := filtered["Base de dados"]; ok {
		t.Fatal("login sheet comments must not be cached")
	}
	if _, ok := filtered["Controle interno"]; ok {
		t.Fatal("unconfigured sheet comments must not be cached")
	}
}

func TestWorkbookCommentsFilterAllowsRuntimeRequestedV2Sheets(t *testing.T) {
	input := map[string][]workbookCellComment{
		"projeto": {
			{SheetName: "projeto", Text: "feedback v2"},
		},
		"Projeto AB2": {
			{SheetName: "Projeto AB2", Text: "feedback legado"},
		},
	}

	filtered := filterWorkbookComments(input, sheetNameSet([]string{"projeto"}))

	if len(filtered) != 1 || len(filtered["projeto"]) != 1 {
		t.Fatalf("filtered = %#v, want dynamic v2 sheet comments", filtered)
	}
	if _, ok := filtered["Projeto AB2"]; ok {
		t.Fatal("unrequested legacy sheet should not be returned")
	}
}

func writeTestZipFile(t *testing.T, writer *zip.Writer, name string, body string) {
	t.Helper()
	file, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
}
