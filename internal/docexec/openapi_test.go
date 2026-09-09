package docexec

import (
	"net/http"
	"strings"
	"testing"
)

func testOpenAPI(t *testing.T) *OpenAPIValidator {
	t.Helper()
	v, err := LoadOpenAPI("../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	return v
}

func TestOpenAPIValidRequestAndResponse(t *testing.T) {
	v := testOpenAPI(t)
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/v1/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateRequest(req); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	body := `{"service":"notrios","version":"0.7","status":"running","search_sidecar":{"configured":false,"available":false,"active":false,"state":"disabled","backlog":0,"failed_jobs":0}}`
	if err := v.ValidateResponse(req, http.StatusOK, http.Header{"Content-Type": []string{"application/json"}}, []byte(body)); err != nil {
		t.Fatalf("valid response: %v", err)
	}
}

func TestOpenAPIMutationsFail(t *testing.T) {
	v := testOpenAPI(t)
	tests := []struct {
		name string
		req  *http.Request
	}{
		{name: "request body type", req: mustRequest(t, http.MethodPost, "http://127.0.0.1:8080/api/v1/collections", `{"id":42,"name":"n","kind":"k"}`)},
		{name: "required confirmation", req: mustRequest(t, http.MethodDelete, "http://127.0.0.1:8080/api/v1/resources/r1", "")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := v.ValidateRequest(tc.req); err == nil {
				t.Fatal("mutation unexpectedly validated")
			}
		})
	}
	req := mustRequest(t, http.MethodGet, "http://127.0.0.1:8080/api/v1/status", "")
	if err := v.ValidateResponse(req, http.StatusOK, http.Header{"Content-Type": []string{"application/json"}}, []byte(`{"status":42}`)); err == nil {
		t.Fatal("invalid response shape unexpectedly validated")
	}
	if err := v.ValidateResponse(req, http.StatusCreated, nil, nil); err == nil {
		t.Fatal("undocumented response status unexpectedly validated")
	}
}

func mustRequest(t *testing.T, method, url, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}
