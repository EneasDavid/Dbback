package app

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/sheets/v4"
)

func TestSheetCacheTTLRefreshesV2GradeSheetsQuickly(t *testing.T) {
	client := &SheetsClient{cfg: Config{
		CacheTTL:       7 * time.Hour,
		LoginSheet:     "Base de dados",
		RuntimeVersion: "v2",
	}}

	for _, sheetName := range []string{v2ABsSheet, v2ActivitiesSheet, "nota ab2", "Pré entrega"} {
		if got := client.sheetCacheTTL(sheetName); got != v2GradeSheetCacheTTL {
			t.Fatalf("sheetCacheTTL(%q) = %s, want %s", sheetName, got, v2GradeSheetCacheTTL)
		}
	}
	if got := client.sheetCacheTTL("Base de dados"); got != 7*time.Hour {
		t.Fatalf("sheetCacheTTL(login) = %s, want 7h", got)
	}
}

func TestSheetCacheTTLKeepsLegacyActivityCache(t *testing.T) {
	client := &SheetsClient{cfg: Config{
		CacheTTL:       7 * time.Hour,
		LoginSheet:     "Base de dados",
		RuntimeVersion: "legacy",
	}}

	if got := client.sheetCacheTTL("Pré entrega"); got != 7*time.Hour {
		t.Fatalf("sheetCacheTTL(legacy activity) = %s, want 7h", got)
	}
}

func TestCommentsCacheTTLRefreshesV2CommentsQuickly(t *testing.T) {
	v2Client := &SheetsClient{cfg: Config{CacheTTL: 7 * time.Hour, RuntimeVersion: "v2"}}
	if got := v2Client.commentsCacheTTL(); got != v2CommentsCacheTTL {
		t.Fatalf("commentsCacheTTL(v2) = %s, want %s", got, v2CommentsCacheTTL)
	}

	legacyClient := &SheetsClient{cfg: Config{CacheTTL: 7 * time.Hour, RuntimeVersion: "legacy"}}
	if got := legacyClient.commentsCacheTTL(); got != 7*time.Hour {
		t.Fatalf("commentsCacheTTL(legacy) = %s, want 7h", got)
	}
}

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

func TestRequiresDriveCommentsSkipsControlSheets(t *testing.T) {
	if requiresDriveComments([]string{"Base de dados", v2ABsSheet, v2ActivitiesSheet}, "Base de dados") {
		t.Fatal("control sheets should not require Drive/XLSX comments")
	}
	if !requiresDriveComments([]string{"nota ab1"}, "Base de dados") {
		t.Fatal("grade sheets should still allow Drive/XLSX comments")
	}
}

func TestSheetNameMatchesLegacyActivityTabsWithDescriptions(t *testing.T) {
	cases := []struct {
		configured string
		actual     string
		want       bool
	}{
		{"AT. 4", "AT. 4 (Álgebra Relacional)", true},
		{"AT. 5", "AT.5 (Mapeamento)", true},
		{"AT. 6", "AT. 6 (SQL)", true},
		{"Projeto AB2", "Acompanhamento Projeto AB2", true},
		{"AT. 1", "AT. 10", false},
	}

	for _, tc := range cases {
		if got := sheetNameMatches(tc.configured, tc.actual); got != tc.want {
			t.Fatalf("sheetNameMatches(%q, %q) = %t, want %t", tc.configured, tc.actual, got, tc.want)
		}
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
