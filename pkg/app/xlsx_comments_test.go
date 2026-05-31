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
