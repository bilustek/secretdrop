package handler

import "net/http"

var openAPISpec []byte

const docsHTML = `<!DOCTYPE html>
<html>
<head>
  <title>SecretDrop API</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/docs/openapi.yaml"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

// SetOpenAPISpec sets the embedded OpenAPI spec bytes.
// Must be called before registering routes.
func SetOpenAPISpec(spec []byte) {
	openAPISpec = spec
}

// RegisterDocs registers the API documentation routes on the given mux.
// If protect is not nil, it wraps the handlers with that middleware (e.g. BasicAuth).
func RegisterDocs(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	spec := http.HandlerFunc(handleOpenAPISpec)
	ui := http.HandlerFunc(handleDocsUI)

	if protect != nil {
		mux.Handle("GET /docs/openapi.yaml", protect(spec))
		mux.Handle("GET /docs", protect(ui))
	} else {
		mux.HandleFunc("GET /docs/openapi.yaml", handleOpenAPISpec)
		mux.HandleFunc("GET /docs", handleDocsUI)
	}
}

func handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, _ = w.Write(openAPISpec)
}

func handleDocsUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(docsHTML))
}
