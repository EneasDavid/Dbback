package app

import "testing"

func TestDecodeDriveCommentsIncludesReplies(t *testing.T) {
	payload, err := decodeDriveComments([]byte(`{
		"comments": [{
			"content": "",
			"anchor": "{\"uid\":0,\"a\":[{\"matrix\":{\"r\":2,\"c\":1}}]}",
			"author": {"displayName": "Professor"},
			"quotedFileContent": {"value": "0,3"},
			"replies": [{
				"content": "feedback da reply",
				"author": {"displayName": "Monitor"}
			}]
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.comments) != 1 {
		t.Fatalf("comments length = %d, want 1", len(payload.comments))
	}
	comment := payload.comments[0]
	if comment.Text != "feedback da reply" {
		t.Fatalf("reply text = %q, want feedback da reply", comment.Text)
	}
	if comment.Author != "Monitor" {
		t.Fatalf("reply author = %q, want Monitor", comment.Author)
	}
	if !comment.HasCell || comment.RowIndex != 2 || comment.ColumnIndex != 1 {
		t.Fatalf("reply cell = (%d, %d, %v), want (2, 1, true)", comment.RowIndex, comment.ColumnIndex, comment.HasCell)
	}
}

func TestDriveCommentCellParsesNestedMatrixAnchor(t *testing.T) {
	rowIdx, colIdx, ok := driveCommentCell(`{"uid":"123","a":[{"matrix":{"r":"4","c":"7"}}]}`)
	if !ok {
		t.Fatal("cell anchor was not detected")
	}
	if rowIdx != 4 || colIdx != 7 {
		t.Fatalf("cell = (%d, %d), want (4, 7)", rowIdx, colIdx)
	}
}
