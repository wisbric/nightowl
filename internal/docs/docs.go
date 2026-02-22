package docs

import (
	_ "embed"
	"net/http"

	"github.com/wisbric/nightowl/docs/api"
)

//go:embed swagger.html
var swaggerHTML []byte

// SwaggerUIHandler serves the Swagger UI page.
func SwaggerUIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(swaggerHTML)
	}
}

// OpenAPISpecHandler serves the OpenAPI YAML spec.
func OpenAPISpecHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(api.OpenAPISpec)
	}
}
