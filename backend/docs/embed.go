package docs

import _ "embed"

// OpenAPISpec holds the raw OpenAPI 3.1 specification.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
