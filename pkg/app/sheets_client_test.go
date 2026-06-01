package app

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/sheets/v4"
)

func TestOptionalDriveCommentsDoesNotBlockSheetsAccess(t *testing.T) {
	client := &SheetsClient{
		cfg: Config{
			SpreadsheetID: "sheet-test-id",
			LoginSheet:    "Base de dados",
		},
		httpClient: &http.Client{Transport: failingRoundTripper{}},
	}

	if got := client.optionalDriveComments(t.Context(), "sheet-test-id", []string{"AT. 1"}); got != nil {
		t.Fatalf("optionalDriveComments() = %#v, want nil fallback", got)
	}
}

func TestSheetReadErrorExplainsServiceAccountPermission(t *testing.T) {
	err := sheetReadError(&googleapi.Error{Code: http.StatusForbidden})

	var httpErr HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("sheetReadError() type = %T, want HTTPError", err)
	}
	if httpErr.Status != http.StatusServiceUnavailable {
		t.Fatalf("sheetReadError() status = %d, want %d", httpErr.Status, http.StatusServiceUnavailable)
	}
	if !strings.Contains(httpErr.Message, "service account") || !strings.Contains(httpErr.Message, "client_email") {
		t.Fatalf("sheetReadError() message = %q, want permission guidance", httpErr.Message)
	}
}

func TestSheetReadErrorExplainsMissingSpreadsheetID(t *testing.T) {
	err := sheetReadError(&googleapi.Error{Code: http.StatusNotFound})

	var httpErr HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("sheetReadError() type = %T, want HTTPError", err)
	}
	if httpErr.Status != http.StatusNotFound {
		t.Fatalf("sheetReadError() status = %d, want %d", httpErr.Status, http.StatusNotFound)
	}
	if !strings.Contains(httpErr.Message, "GOOGLE_SHEET_ID") {
		t.Fatalf("sheetReadError() message = %q, want spreadsheet ID guidance", httpErr.Message)
	}
}

func TestSchemaStatusForSpreadsheetMarksLegacyWhenMetadataDiffers(t *testing.T) {
	client := &SheetsClient{cfg: Config{RuntimeVersion: "v2", MetadataKey: "dbback_schema", MetadataValue: "v2"}}

	got := client.schemaStatusForSpreadsheet([]*sheets.DeveloperMetadata{
		{MetadataKey: "dbback_schema", MetadataValue: "v1"},
	})

	if got != "legacy" {
		t.Fatalf("schemaStatusForSpreadsheet() = %q, want legacy", got)
	}
}

func TestSchemaStatusForSpreadsheetAcceptsConfiguredV2Metadata(t *testing.T) {
	client := &SheetsClient{cfg: Config{RuntimeVersion: "v2", MetadataKey: "dbback_schema", MetadataValue: "v2"}}

	got := client.schemaStatusForSpreadsheet([]*sheets.DeveloperMetadata{
		{MetadataKey: "dbback_schema", MetadataValue: "v2"},
	})

	if got != "v2" {
		t.Fatalf("schemaStatusForSpreadsheet() = %q, want v2", got)
	}
}

func TestSchemaStatusForSpreadsheetLeavesMissingMetadataUndecided(t *testing.T) {
	client := &SheetsClient{cfg: Config{RuntimeVersion: "v2", MetadataKey: "dbback_schema", MetadataValue: "v2"}}

	if got := client.schemaStatusForSpreadsheet(nil); got != "" {
		t.Fatalf("schemaStatusForSpreadsheet() = %q, want undecided", got)
	}
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("drive unavailable")
}
