// Package openapi provides the embedded OpenAPI specification.
package openapi

import _ "embed"

// V1Spec is the OpenAPI 3.0 specification for the EdgeFabric v1 API.
//
//go:embed v1.yaml
var V1Spec []byte
