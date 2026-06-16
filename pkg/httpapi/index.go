package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"feedback/pkg/app"
)

type Router struct {
	Auth    AuthController
	Passkey PasskeyController
	Grades  GradesController
	Docs    DocsController
}

func NewRouter() Router {
	return Router{
		Auth:    AuthController{},
		Passkey: PasskeyController{},
		Grades:  GradesController{},
		Docs:    DocsController{},
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	NewRouter().ServeHTTP(w, r)
}

func (router Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := apiPath(r)
	switch {
	case path == "/api/login":
		router.Auth.Login(w, r)
	case path == "/api/logout":
		router.Auth.Logout(w, r)
	case path == "/api/me":
		router.Auth.Me(w, r)
	case path == "/api/passkey/register/options":
		router.Passkey.RegisterOptions(w, r)
	case path == "/api/passkey/register":
		router.Passkey.Register(w, r)
	case path == "/api/passkey/login/options":
		router.Passkey.LoginOptions(w, r)
	case path == "/api/passkey/login":
		router.Passkey.Login(w, r)
	case path == "/api/grades/all":
		router.Grades.All(w, r)
	case path == "/api/grades" || strings.HasPrefix(path, "/api/grades/"):
		router.Grades.Show(w, r, path)
	case path == "/api/docs":
		router.Docs.Show(w, r)
	default:
		app.Error(w, app.NewHTTPError(http.StatusNotFound, "rota nao encontrada"))
	}
}

func apiPath(r *http.Request) string {
	path := requestPath(r)
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return "/"
	}
	if (path == "/api" || path == "/api/index" || path == "/api/index.go") && r.Method == http.MethodGet {
		return "/api/docs"
	}
	if strings.HasPrefix(path, "/api/index.go/") {
		return "/api/" + strings.TrimPrefix(path, "/api/index.go/")
	}
	if strings.HasPrefix(path, "/api/index/") {
		return "/api/" + strings.TrimPrefix(path, "/api/index/")
	}
	return path
}

func requestPath(r *http.Request) string {
	for _, header := range []string{"X-Vercel-Original-Url", "X-Original-URL", "X-Rewrite-URL", "X-Forwarded-Uri"} {
		if path := pathFromHeader(r.Header.Get(header)); path != "" {
			return path
		}
	}
	return r.URL.Path
}

func pathFromHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && strings.HasPrefix(parsed.Path, "/api/") {
		return parsed.Path
	}
	if strings.HasPrefix(value, "/api/") {
		if idx := strings.IndexAny(value, "?#"); idx >= 0 {
			return value[:idx]
		}
		return value
	}
	return ""
}
