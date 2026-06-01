package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionKeepsSpreadsheetID(t *testing.T) {
	manager := SessionManager{secret: []byte("test-secret")}
	recorder := httptest.NewRecorder()
	manager.Set(recorder, SessionUser{Matricula: "123", Name: "Alice", SpreadsheetID: "sheet-new", SchemaStatus: "v2"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		req.AddCookie(cookie)
	}

	user, ok := manager.User(req)
	if !ok {
		t.Fatal("User() ok = false, want true")
	}
	if user.SpreadsheetID != "sheet-new" {
		t.Fatalf("SpreadsheetID = %q, want sheet-new", user.SpreadsheetID)
	}
	if user.SchemaStatus != "v2" {
		t.Fatalf("SchemaStatus = %q, want v2", user.SchemaStatus)
	}
}
