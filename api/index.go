package handler

import (
	"net/http"

	"feedback/pkg/httpapi"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	httpapi.Handler(w, r)
}
