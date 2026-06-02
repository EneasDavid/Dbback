package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"feedback/pkg/app"
)

type DocsController struct{}

func (DocsController) Show(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodGet) {
		return
	}
	if !docsAuthorized(w, r) {
		return
	}
	payload := docsPayload()
	if wantsDocsHTML(r) {
		renderDocsHTML(w, payload)
		return
	}
	app.JSON(w, http.StatusOK, payload)
}

func wantsDocsHTML(r *http.Request) bool {
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "json") {
		return false
	}
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json")
}

func docsPayload() map[string]any {
	return map[string]any{
		"name":        "dbBack",
		"type":        "Go + React monolith",
		"description": "Consulta autenticada de notas e feedbacks a partir de Google Sheets/Drive.",
		"version": map[string]any{
			"label":     "v2-stable",
			"v2_stable": true,
		},
		"routes": []map[string]any{
			{
				"method": "POST",
				"path":   "/api/login",
				"auth":   false,
				"body":   map[string]string{"matricula": "string"},
				"response": map[string]string{
					"matricula":    "string",
					"name":         "string",
					"schemaStatus": "legacy|v2 optional",
				},
				"result": "Cria sessão assinada depois de validar a matrícula na aba Base de dados de uma ou mais planilhas configuradas.",
				"cache":  "no-store",
			},
			{
				"method": "POST",
				"path":   "/api/logout",
				"auth":   false,
				"response": map[string]string{
					"ok": "boolean",
				},
				"result": "Remove o cookie de sessão.",
				"cache":  "no-store",
			},
			{
				"method": "GET",
				"path":   "/api/me",
				"auth":   false,
				"response": map[string]any{
					"authenticated": map[string]string{"matricula": "string", "name": "string"},
					"anonymous":     nil,
				},
				"result": "Retorna o usuário da sessão atual ou null.",
				"cache":  "no-store",
			},
			{
				"method": "GET",
				"path":   "/api/grades",
				"aliases": []string{
					"/api/grades?exam=ab1",
					"/api/grades?exam=ab2",
					"/api/grades?exam=<ab-da-aba-abs>",
					"/api/grades/ab1",
					"/api/grades/ab2",
					"/api/grades/<ab-da-aba-abs>",
					"/api/index.go/grades?exam=<ab-da-aba-abs>",
				},
				"auth":   true,
				"result": "Retorna tabelas render-ready da avaliação solicitada. Na v2, a avaliação deve existir na aba abs e só é renderizada quando status/ativo é 1.",
				"query": map[string]string{
					"exam":    "ab1, ab2 ou qualquer chave ativa v2 vinda da aba abs; aceita ab1|ab2 e usa o primeiro valor válido",
					"refresh": "1 opcional; limpa cache em memória",
				},
				"cache":    "private, max-age=30, stale-while-revalidate=300, ETag e Vary: Cookie, Accept-Encoding",
				"response": gradeResponseSchema(),
			},
			{
				"method":   "GET",
				"path":     "/api/grades/all",
				"auth":     true,
				"aliases":  []string{"/api/index.go/grades/all"},
				"result":   "Retorna todas as avaliações disponíveis. Na v2, usa somente ABs ativas da aba abs; no legado, retorna AB1/AB2.",
				"query":    map[string]string{"refresh": "1 opcional; limpa cache em memória"},
				"cache":    "private, max-age=30, stale-while-revalidate=300, ETag e Vary: Cookie, Accept-Encoding",
				"response": map[string]any{"<examKey>": gradeResponseSchema()},
			},
			{
				"method": "GET",
				"path":   "/api/docs",
				"aliases": []string{
					"/api",
					"/api/index",
					"/api/index.go",
					"/api/docs?format=json",
					"/api/index.go/docs?format=json",
				},
				"auth":  true,
				"query": map[string]string{"format": "json opcional; força resposta JSON quando Accept pede HTML"},
				"response": map[string]string{
					"name":              "string",
					"type":              "string",
					"description":       "string",
					"version":           "object",
					"routes":            "array<object>",
					"gradeOrganization": "object",
					"network":           "object",
					"paa":               "object",
					"dataFlowGraph":     "object",
				},
				"result": "Documentação técnica de todas as rotas HTTP públicas e aliases serverless.",
				"cache":  "no-store",
			},
		},
		"gradeOrganization": map[string]any{
			"identitySource": "Base de dados",
			"rowSelection":   "A matrícula resolve uma identidade e fixa a planilha internamente na sessão assinada. O spreadsheetId não é exposto nas respostas públicas.",
			"feedbackSource": "Comentários vêm de cell notes, workbook/XLSX comments e Drive comments quando a service account consegue enxergar e mapear célula. Na v2, comentários de critério entram em Detail.comment; comentários da célula da nota final/atividade entram no card.",
			"rendering":      "O backend monta cards/detalhes render-ready. Critérios v2 usam a escala da rubrica; a nota da atividade vem da aba nota <ab> e médias são limitadas a 10.",
			"versioning":     "SHEETS_RUNTIME_VERSION=v2/auto usa a aba abs e developer metadata. Em /api/grades/all, só ABs com status ativo=1 são retornadas na v2.",
		},
		"security": map[string]any{
			"readOnlySheets": "A service account é criada com escopos somente leitura para Sheets e Drive; não há chamada de update, append ou batchUpdate.",
			"publicPayload":  "Respostas removem spreadsheetId e nunca expõem linhas completas: só a linha do aluno autenticado e cards/detalhes tipados.",
			"inputControl":   "POST aceita apenas login/logout, valida origem, limita body de login a 1 KiB e aceita matrícula numérica.",
			"sqlInjection":   "A aplicação não usa banco SQL nem monta queries SQL; entradas são normalizadas antes de escolher rotas/abas permitidas.",
			"integrity":      "Sessões são HMAC-SHA256 e respostas JSON incluem digest SHA-256 para verificação tamper-evident, inspirado em cadeias de hash.",
		},
		"network": map[string]any{
			"performanceBudget": "Meta operacional: login + primeiro render de notas em até 2s no caminho quente. Caminho quente significa runtime Vercel já inicializado, planilha e comentários dentro do TTL e navegador com cache SWR local.",
			"httpClient":        "Cliente Google com timeout total de 15s, keep-alive, pool de conexões e timeout de cabeçalho/TLS para evitar conexões presas.",
			"connectionReuse":   "O mesmo SheetsClient é mantido por runtime e os escopos por planilha compartilham cache/singleflight, reaproveitando conexões, tokens e leituras já feitas.",
			"batching":          "Leituras do Google Sheets são agrupadas por range em uma chamada sempre que possível.",
			"serverCache":       "Cache em memória por aba, planilha e comentários com TTL configurado em CacheTTL; refresh=1 limpa o cache do processo.",
			"httpCache":         "GET /api/grades e /api/grades/all retornam ETag, Cache-Control private max-age=30 stale-while-revalidate=300 e Vary: Cookie, Accept-Encoding.",
			"clientSWR":         "O frontend renderiza sessionStorage imediatamente, revalida em background com If-None-Match e preserva o payload antigo quando o servidor responde 304.",
			"prefetch":          "Cards de atividade disparam prefetch em hover e foco, antes do clique de expansão.",
			"deduplication":     "singleflight no backend e dedupe de GET em voo no frontend evitam leituras duplicadas quando requisições simultâneas pedem as mesmas notas.",
			"compression":       "Payloads JSON são textuais, têm Vary: Accept-Encoding e são elegíveis à compressão Brotli/Gzip automática da Vercel; localmente o foco é medir payload e cache.",
			"payloadControl":    "Fields restrito na Sheets API, export XLSX limitado a 25MiB e respostas públicas removem spreadsheetId.",
		},
		"paa": map[string]any{
			"plano":     "Resolver a matrícula em Base de dados, fixar spreadsheetId apenas na sessão assinada e escolher parser legado/v2 por runtime/schemaStatus.",
			"acesso":    "Validar cookie HMAC, aceitar POST apenas same-origin, usar service account read-only e nunca expor credenciais ou spreadsheetId no payload público.",
			"auditoria": "Publicar schemaStatus, Digest SHA-256, ETag e X-Dbback-Content-SHA256 para rastrear versão, integridade e revalidação sem revelar dados sensíveis.",
			"acao":      "Renderizar somente a linha do aluno autenticado, normalizada em cards/detalhes tipados para a UI.",
		},
		"dataFlowGraph": map[string]any{
			"nodes": []string{
				"Browser LoginView",
				"AppController",
				"AuthController",
				"GradesController",
				"SheetsClient cache/singleflight",
				"Google Sheets/Drive",
				"GradeModel normalizer",
				"Views GradeCard/DetailPanel",
			},
			"edges": []string{
				"Browser LoginView -> AuthController: POST /api/login",
				"AuthController -> SheetsClient: LoginIdentity(Base de dados)",
				"AuthController -> SheetsClient: warm-up assíncrono de GradesFor",
				"AppController -> GradesController: GET /api/grades/all com If-None-Match",
				"GradesController -> SheetsClient cache/singleflight -> Google Sheets/Drive",
				"GradesController -> Browser: JSON privado com ETag",
				"GradeModel normalizer -> Views GradeCard/DetailPanel: cards e subtópicos render-ready",
			},
		},
	}
}

func renderDocsHTML(w http.ResponseWriter, payload map[string]any) {
	data := docsHTMLData{
		Name:        stringValue(payload["name"]),
		Type:        stringValue(payload["type"]),
		Description: stringValue(payload["description"]),
		Routes:      docsHTMLRoutes(payload["routes"]),
		GradeFacts:  docsHTMLFacts(payload["gradeOrganization"], gradeFactOrder()),
		SecurityFacts: docsHTMLFacts(payload["security"], []docsHTMLFactKey{
			{key: "readOnlySheets", label: "Planilha somente leitura"},
			{key: "publicPayload", label: "Payload público"},
			{key: "inputControl", label: "Entrada controlada"},
			{key: "sqlInjection", label: "SQL injection"},
			{key: "integrity", label: "Integridade"},
		}),
		NetworkFacts: docsHTMLFacts(payload["network"], []docsHTMLFactKey{
			{key: "performanceBudget", label: "Orçamento de performance"},
			{key: "httpClient", label: "Cliente HTTP"},
			{key: "connectionReuse", label: "Reuso de conexão"},
			{key: "batching", label: "Batching"},
			{key: "serverCache", label: "Cache servidor"},
			{key: "httpCache", label: "Cache HTTP"},
			{key: "clientSWR", label: "SWR no navegador"},
			{key: "prefetch", label: "Prefetch"},
			{key: "deduplication", label: "Deduplicação"},
			{key: "compression", label: "Compressão"},
			{key: "payloadControl", label: "Controle de payload"},
		}),
		PAAFacts: docsHTMLFacts(payload["paa"], []docsHTMLFactKey{
			{key: "plano", label: "Plano"},
			{key: "acesso", label: "Acesso"},
			{key: "auditoria", label: "Auditoria"},
			{key: "acao", label: "Ação"},
		}),
		GraphFacts: docsHTMLFacts(docsGraphFacts(payload["dataFlowGraph"]), []docsHTMLFactKey{
			{key: "nodes", label: "Nós"},
			{key: "edges", label: "Arestas"},
		}),
	}
	for idx := range data.Routes {
		data.Routes[idx].Schema = prettyJSON(data.Routes[idx].Response)
		data.Routes[idx].BodySchema = prettyJSON(data.Routes[idx].Body)
		data.Routes[idx].QuerySchema = prettyJSON(data.Routes[idx].Query)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	app.SecureHeaders(w)
	w.WriteHeader(http.StatusOK)
	if err := docsTemplate.Execute(w, data); err != nil {
		app.Error(w, err)
	}
}

type docsHTMLData struct {
	Name          string
	Type          string
	Description   string
	Routes        []docsHTMLRoute
	GradeFacts    []docsHTMLFact
	SecurityFacts []docsHTMLFact
	NetworkFacts  []docsHTMLFact
	PAAFacts      []docsHTMLFact
	GraphFacts    []docsHTMLFact
}

type docsHTMLRoute struct {
	Method      string
	Path        string
	Auth        bool
	Result      string
	Cache       string
	Body        any
	Query       any
	Aliases     []string
	Response    any
	Schema      string
	BodySchema  string
	QuerySchema string
}

type docsHTMLFact struct {
	Label string
	Value string
}

func docsHTMLRoutes(value any) []docsHTMLRoute {
	items, ok := value.([]map[string]any)
	if !ok {
		return nil
	}
	routes := make([]docsHTMLRoute, 0, len(items))
	for _, item := range items {
		routes = append(routes, docsHTMLRoute{
			Method:   stringValue(item["method"]),
			Path:     stringValue(item["path"]),
			Auth:     boolValue(item["auth"]),
			Result:   stringValue(item["result"]),
			Cache:    stringValue(item["cache"]),
			Body:     item["body"],
			Query:    item["query"],
			Aliases:  stringSlice(item["aliases"]),
			Response: item["response"],
		})
	}
	return routes
}

type docsHTMLFactKey struct {
	key   string
	label string
}

func gradeFactOrder() []docsHTMLFactKey {
	return []docsHTMLFactKey{
		{key: "identitySource", label: "Identidade"},
		{key: "rowSelection", label: "Seleção da linha"},
		{key: "feedbackSource", label: "Feedback"},
		{key: "rendering", label: "Renderização"},
		{key: "versioning", label: "Versões"},
	}
}

func docsHTMLFacts(value any, order []docsHTMLFactKey) []docsHTMLFact {
	items, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	facts := make([]docsHTMLFact, 0, len(order))
	for _, item := range order {
		facts = append(facts, docsHTMLFact{Label: item.label, Value: stringValue(items[item.key])})
	}
	return facts
}

func docsGraphFacts(value any) map[string]any {
	graph, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return map[string]any{
		"nodes": strings.Join(stringSlice(graph["nodes"]), " -> "),
		"edges": strings.Join(stringSlice(graph["edges"]), " | "),
	}
}

func prettyJSON(value any) string {
	if value == nil {
		return "null"
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(content)
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}

func stringSlice(value any) []string {
	items, ok := value.([]string)
	if !ok {
		return nil
	}
	return items
}

func gradeResponseSchema() map[string]any {
	return map[string]any{
		"exam":         "string",
		"matricula":    "string",
		"name":         "string",
		"schemaStatus": "legacy|v2 optional",
		"tables": []map[string]any{
			{
				"key":          "string",
				"label":        "string",
				"sheetName":    "string",
				"kind":         "string",
				"complete":     "boolean",
				"status":       "string optional",
				"schemaStatus": "legacy|v2 optional",
				"cards": []map[string]any{
					{
						"key":           "string",
						"label":         "string",
						"value":         "string",
						"displayValue":  "string",
						"tone":          "string optional",
						"comment":       "string optional",
						"commentAuthor": "string optional",
						"details": []map[string]string{
							{
								"key":           "string",
								"label":         "string",
								"value":         "string",
								"max":           "number",
								"displayScore":  "string",
								"ratio":         "number",
								"pending":       "boolean",
								"tone":          "string optional",
								"comment":       "string optional",
								"commentAuthor": "string optional",
							},
						},
					},
				},
			},
		},
		"studentStatus": "object optional",
	}
}

func docsAuthorized(w http.ResponseWriter, r *http.Request) bool {
	cfg := app.LoadConfig()
	if strings.TrimSpace(cfg.DocsUsername) == "" || strings.TrimSpace(cfg.DocsPassword) == "" {
		app.Error(w, app.NewHTTPError(http.StatusInternalServerError, "credenciais das docs nao configuradas no ambiente"))
		return false
	}
	username, password, ok := r.BasicAuth()
	if ok && secureCompare(username, cfg.DocsUsername) && secureCompare(password, cfg.DocsPassword) {
		return true
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="dbBack docs", charset="UTF-8"`)
	app.Error(w, app.NewHTTPError(http.StatusUnauthorized, "autenticacao obrigatoria"))
	return false
}

func secureCompare(left string, right string) bool {
	leftHash := sha256.Sum256([]byte(left))
	rightHash := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftHash[:], rightHash[:]) == 1
}

var docsTemplate = template.Must(template.New("docs").Parse(`<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex, nofollow">
  <title>{{.Name}} Documentação da API</title>
  <style>
    :root {
      color-scheme: light dark;
      --bg: #f5f7fb;
      --panel: #ffffff;
      --panel-soft: #eef3f8;
      --text: #18202f;
      --muted: #5e6c81;
      --line: #d9e1ec;
      --accent: #145A3A;
      --accent-soft: #e4f2e9;
      --code: #101828;
      --code-bg: #f0f4f8;
      --danger: #9f3a43;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #202823;
        --panel: #303a34;
        --panel-soft: #39453d;
        --text: #f3f1ea;
        --muted: #bcc7bd;
        --line: #4b574f;
        --accent: #145A3A;
        --accent-soft: #1b2e24;
        --code: #f4f1e8;
        --code-bg: #1a211d;
        --danger: #e4aaa5;
      }
    }
    * { box-sizing: border-box; }
    body { margin: 0; background: var(--bg); color: var(--text); line-height: 1.5; }
    a { color: inherit; }
    .page {
      width: min(1120px, calc(100% - 32px));
      margin: 0 auto;
      padding: 40px 0 56px;
    }
    .hero {
      display: grid;
      gap: 18px;
      padding: 28px 0 22px;
    }
    .eyebrow {
      color: var(--accent);
      font-size: 13px;
      font-weight: 800;
      letter-spacing: 0;
      text-transform: uppercase;
    }
    h1 {
      margin: 0;
      font-size: clamp(32px, 7vw, 72px);
      line-height: .94;
      letter-spacing: 0;
    }
    .hero p {
      max-width: 760px;
      margin: 0;
      color: var(--muted);
      font-size: 18px;
    }
    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-top: 6px;
    }
    .button {
      display: inline-flex;
      align-items: center;
      min-height: 42px;
      padding: 0 14px;
      border: 1px solid var(--line);
      border-radius: 7px;
      background: var(--panel);
      color: var(--text);
      font-weight: 700;
      text-decoration: none;
    }
    .button.primary {
      border-color: transparent;
      background: var(--accent);
      color: #ffffff;
    }
    .overview {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
      gap: 12px;
      margin: 24px 0;
    }
    .metric, .route, .fact {
      border: 1px solid var(--line);
      border-radius: 8px;
      background: color-mix(in srgb, var(--panel) 94%, transparent);
      box-shadow: 0 20px 50px rgba(15, 23, 42, .08);
    }
    .metric {
      padding: 16px;
    }
    .metric span {
      display: block;
      color: var(--muted);
      font-size: 13px;
      font-weight: 700;
    }
    .metric strong {
      display: block;
      margin-top: 6px;
      font-size: 20px;
    }
    .section-title {
      display: flex;
      align-items: end;
      justify-content: space-between;
      gap: 16px;
      margin: 34px 0 14px;
    }
    .section-title h2 {
      margin: 0;
      font-size: 24px;
    }
    .section-title p {
      max-width: 560px;
      margin: 0;
      color: var(--muted);
      font-size: 14px;
    }
    .routes {
      display: grid;
      gap: 12px;
    }
    .route {
      overflow: hidden;
    }
    .route-header {
      display: grid;
      grid-template-columns: auto 1fr auto;
      gap: 10px;
      align-items: center;
      padding: 16px;
      border-bottom: 1px solid var(--line);
      background: var(--panel);
    }
    .method, .auth {
      border-radius: 999px;
      padding: 5px 8px;
      font-size: 12px;
      font-weight: 900;
    }
    .method {
      background: var(--accent-soft);
      color: var(--accent);
    }
    .auth {
      border: 1px solid var(--line);
      color: var(--muted);
    }
    .auth.required {
      color: var(--danger);
    }
    code {
      color: var(--code);
      font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
      font-size: 13px;
      overflow-wrap: anywhere;
    }
    .route-body {
      display: grid;
      grid-template-columns: minmax(0, .9fr) minmax(0, 1.1fr);
      gap: 16px;
      padding: 16px;
    }
    .route-copy {
      margin: 0 0 12px;
      color: var(--muted);
    }
    .chips {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-top: 10px;
    }
    .chip {
      min-width: 0;
      border: 1px solid var(--line);
      border-radius: 999px;
      padding: 6px 9px;
      background: var(--panel-soft);
    }
    .schema-label {
      display: block;
      margin-top: 14px;
      margin-bottom: 6px;
    }
    .schema {
      min-width: 0;
      max-width: 100%;
      margin: 0;
      overflow: auto;
      border-radius: 7px;
      padding: 14px;
      background: var(--code-bg);
      color: var(--code);
      font-size: 12px;
      line-height: 1.55;
    }
    .schema.compact {
      margin-bottom: 10px;
      padding: 10px;
    }
    .facts {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
      gap: 12px;
    }
    .fact {
      padding: 16px;
    }
    .fact strong {
      display: block;
      margin-bottom: 8px;
      font-size: 15px;
    }
    .fact p {
      margin: 0;
      color: var(--muted);
    }
    @media (max-width: 780px) {
      .page { width: min(100% - 20px, 1120px); padding: 18px 0 34px; }
      .hero { gap: 14px; padding: 18px 0; }
      .hero p { font-size: 16px; }
      .route, .metric, .fact { border-radius: 7px; box-shadow: none; }
      .route-body { grid-template-columns: 1fr; padding: 12px; }
      .route-header { grid-template-columns: auto 1fr; }
      .auth { grid-column: 1 / -1; justify-self: start; }
      .section-title { display: block; }
      .section-title p { margin-top: 6px; }
      .button { flex: 1 1 150px; justify-content: center; }
    }
  </style>
</head>
<body>
  <main class="page">
    <section class="hero">
      <div class="eyebrow">{{.Type}}</div>
      <h1>{{.Name}} Documentação da API</h1>
      <p>{{.Description}} Esta página mostra somente schemas e metadados, sem matrícula, nome, nota ou feedback real de alunos.</p>
      <div class="actions">
        <a class="button primary" href="#routes">Ver rotas</a>
        <a class="button" href="/api/docs?format=json">Abrir JSON</a>
      </div>
    </section>

    <section class="overview" aria-label="Resumo">
      <div class="metric"><span>Rotas</span><strong>{{len .Routes}}</strong></div>
      <div class="metric"><span>Autenticação</span><strong>Cookie + Basic</strong></div>
      <div class="metric"><span>Privacidade</span><strong>Campos tipados</strong></div>
      <div class="metric"><span>Cache</span><strong>SWR + ETag</strong></div>
    </section>

    <section id="routes">
      <div class="section-title">
        <h2>Rotas HTTP</h2>
        <p>Os schemas abaixo descrevem o formato das respostas. Dados privados são retornados apenas na sessão autenticada do aluno.</p>
      </div>

      <div class="routes">
        {{range .Routes}}
        <article class="route">
          <div class="route-header">
            <span class="method">{{.Method}}</span>
            <code>{{.Path}}</code>
            {{if .Auth}}<span class="auth required">auth obrigatória</span>{{else}}<span class="auth">sem sessão</span>{{end}}
          </div>
          <div class="route-body">
            <div>
              <p class="route-copy">{{.Result}}</p>
              {{if .Cache}}
              <strong>Cache</strong>
              <p class="route-copy"><code>{{.Cache}}</code></p>
              {{end}}
              {{if .Aliases}}
              <strong>Aliases</strong>
              <div class="chips">
                {{range .Aliases}}<code class="chip">{{.}}</code>{{end}}
              </div>
              {{end}}
              {{if .Body}}
              <strong class="schema-label">Body</strong>
              <pre class="schema compact"><code>{{.BodySchema}}</code></pre>
              {{end}}
              {{if .Query}}
              <strong class="schema-label">Query</strong>
              <pre class="schema compact"><code>{{.QuerySchema}}</code></pre>
              {{end}}
            </div>
            <pre class="schema"><code>{{.Schema}}</code></pre>
          </div>
        </article>
        {{end}}
      </div>
    </section>

    <section>
      <div class="section-title">
        <h2>Organização dos dados</h2>
        <p>Resumo das regras usadas para localizar a linha correta e renderizar o payload no frontend.</p>
      </div>
      <div class="facts">
        {{range .GradeFacts}}
        <article class="fact">
          <strong>{{.Label}}</strong>
          <p>{{.Value}}</p>
        </article>
        {{end}}
      </div>
    </section>

    <section>
      <div class="section-title">
        <h2>Segurança</h2>
        <p>Controles para reduzir vazamento de dados, escrita indevida e abuso de entrada.</p>
      </div>
      <div class="facts">
        {{range .SecurityFacts}}
        <article class="fact">
          <strong>{{.Label}}</strong>
          <p>{{.Value}}</p>
        </article>
        {{end}}
      </div>
    </section>

    <section>
      <div class="section-title">
        <h2>Rede e performance</h2>
        <p>Controles usados para reduzir latência, evitar chamadas duplicadas e manter respostas previsíveis em serverless.</p>
      </div>
      <div class="facts">
        {{range .NetworkFacts}}
        <article class="fact">
          <strong>{{.Label}}</strong>
          <p>{{.Value}}</p>
        </article>
        {{end}}
      </div>
    </section>

    <section>
      <div class="section-title">
        <h2>PAA</h2>
        <p>Plano de Acesso e Auditoria usado para decidir fonte, acesso, rastreabilidade e ação de renderização.</p>
      </div>
      <div class="facts">
        {{range .PAAFacts}}
        <article class="fact">
          <strong>{{.Label}}</strong>
          <p>{{.Value}}</p>
        </article>
        {{end}}
      </div>
    </section>

    <section>
      <div class="section-title">
        <h2>Grafo de dados</h2>
        <p>Fluxo lógico das principais arestas de autenticação, cache, planilha e renderização.</p>
      </div>
      <div class="facts">
        {{range .GraphFacts}}
        <article class="fact">
          <strong>{{.Label}}</strong>
          <p>{{.Value}}</p>
        </article>
        {{end}}
      </div>
    </section>
  </main>
</body>
</html>`))
