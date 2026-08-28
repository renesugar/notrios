// Package docexec contains the executable documentation contract helpers.
package docexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

var registerPublishedBodyDecoders sync.Once

// OpenAPIValidator validates HTTP examples against an OpenAPI document. The
// router is built once so request and response checks resolve the same
// operation, including path parameters.
type OpenAPIValidator struct {
	doc    *openapi3.T
	router routers.Router
}

// LoadOpenAPI loads and validates an OpenAPI 3.x document from path.
func LoadOpenAPI(path string) (*OpenAPIValidator, error) {
	registerPublishedBodyDecoders.Do(func() {
		openapi3filter.RegisterBodyDecoder("text/markdown", openapi3filter.PlainBodyDecoder)
		openapi3filter.RegisterBodyDecoder("image/png", openapi3filter.FileBodyDecoder)
	})
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI document %q: %w", path, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate OpenAPI document %q: %w", path, err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("build OpenAPI router: %w", err)
	}
	return &OpenAPIValidator{doc: doc, router: router}, nil
}

// NewOpenAPIValidator constructs a validator from an already loaded document.
// The document must have been validated by the caller.
func NewOpenAPIValidator(doc *openapi3.T) (*OpenAPIValidator, error) {
	if doc == nil {
		return nil, fmt.Errorf("OpenAPI document is nil")
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("build OpenAPI router: %w", err)
	}
	return &OpenAPIValidator{doc: doc, router: router}, nil
}

// Document returns the parsed contract for callers needing operation metadata.
func (v *OpenAPIValidator) Document() *openapi3.T { return v.doc }

// ValidateRequest checks method, path, parameters, headers, and request body.
// The body is restored after validation, making this safe before an actual
// loopback request is sent.
func (v *OpenAPIValidator) ValidateRequest(req *http.Request) error {
	if v == nil || v.router == nil {
		return fmt.Errorf("OpenAPI validator is nil")
	}
	if req == nil {
		return fmt.Errorf("HTTP request is nil")
	}
	var body []byte
	var err error
	if req.Body != nil {
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("read request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	route, params, err := v.router.FindRoute(req)
	if err != nil {
		return fmt.Errorf("resolve %s %s: %w", req.Method, req.URL.Path, err)
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	input := &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route}
	if err := openapi3filter.ValidateRequest(context.Background(), input); err != nil {
		return fmt.Errorf("validate request %s %s: %w", req.Method, req.URL.Path, err)
	}
	// ValidateRequest consumes the body; callers should receive the original.
	req.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}

// ValidateResponse checks a response against the operation selected by req.
// body may be nil when the operation documents no response content.
func (v *OpenAPIValidator) ValidateResponse(req *http.Request, status int, headers http.Header, body []byte) error {
	if v == nil || v.router == nil {
		return fmt.Errorf("OpenAPI validator is nil")
	}
	if req == nil {
		return fmt.Errorf("HTTP request is nil")
	}
	route, params, err := v.router.FindRoute(req)
	if err != nil {
		return fmt.Errorf("resolve %s %s: %w", req.Method, req.URL.Path, err)
	}
	if route.Operation == nil || route.Operation.Responses == nil ||
		(route.Operation.Responses.Status(status) == nil && route.Operation.Responses.Default() == nil) {
		return fmt.Errorf("validate response %s %s status %d: status is not documented", req.Method, req.URL.Path, status)
	}
	reqInput := &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route}
	response := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 status,
		Header:                 headers,
	}
	response.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(context.Background(), response); err != nil {
		return fmt.Errorf("validate response %s %s status %d: %w", req.Method, req.URL.Path, status, err)
	}
	return nil
}
