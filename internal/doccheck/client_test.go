package doccheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientHealthAndCompletion(t *testing.T) {
	var got map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			if r.Method != http.MethodGet {
				t.Errorf("health method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		if r.URL.Path != "/completion" || r.Method != http.MethodPost {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"content":"answer","model":"qwen","tokens_predicted":4,"tokens_evaluated":8,"stop":true,"stop_type":"eos","timings":{"prompt_ms":1.5,"predicted_ms":2.5}}`))
	}))
	defer s.Close()
	c := Client{Endpoint: s.URL}
	if err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	out, err := c.Complete(context.Background(), CompletionRequest{Prompt: "hello", Temperature: 0.7, NPredict: 12, Stop: []string{"END"}, Grammar: "root ::= \"x\""})
	if err != nil {
		t.Fatal(err)
	}
	if out.Content != "answer" || out.Model != "qwen" || out.TokensPredicted != 4 || out.TokensEvaluated != 8 || !out.Stop || out.StopType != "eos" || out.Timings.PromptMilliseconds != 1.5 || out.Timings.PredictedMilliseconds != 2.5 {
		t.Fatalf("unexpected result: %+v", out)
	}
	if got["prompt"] != "hello" || got["temperature"] != 0.7 || got["n_predict"] != float64(12) || got["grammar"] != `root ::= "x"` {
		t.Fatalf("request fields: %#v", got)
	}
	stops, ok := got["stop"].([]any)
	if !ok || len(stops) != 1 || stops[0] != "END" {
		t.Fatalf("stop: %#v", got["stop"])
	}
}

func TestClientRejectsEndpointsBeforeRequest(t *testing.T) {
	for _, endpoint := range []string{"https://127.0.0.1:1", "http://127.0.0.2:1", "http://localhost.evil:1", "http://user@127.0.0.1:1", "http://127.0.0.1:0", "http://127.0.0.1:65536", "http://127.0.0.1/path", "http://127.0.0.1:bad"} {
		called := false
		c := Client{Endpoint: endpoint, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { called = true; return nil, context.Canceled })}}
		if _, err := c.Complete(context.Background(), CompletionRequest{Prompt: "x", Temperature: 1, NPredict: 1}); err == nil {
			t.Errorf("%s accepted", endpoint)
		}
		if called {
			t.Errorf("%s made request", endpoint)
		}
	}
}

func TestCompletionLimits(t *testing.T) {
	c := Client{Endpoint: "http://127.0.0.1:1"}
	tests := []CompletionRequest{{Temperature: 1, NPredict: 1}, {Prompt: "x", Temperature: -1, NPredict: 1}, {Prompt: "x", Temperature: 3, NPredict: 1}, {Prompt: "x", Temperature: 1, NPredict: 0}, {Prompt: "x", Temperature: 1, NPredict: 4097}, {Prompt: "x", Temperature: 1, NPredict: 1, Stop: []string{""}}, {Prompt: "x", Temperature: 1, NPredict: 1, Grammar: strings.Repeat("g", maxGrammarBytes+1)}}
	for i, req := range tests {
		if _, err := c.Complete(context.Background(), req); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

func TestCompletionHTTPAndStrictErrors(t *testing.T) {
	for _, body := range []string{`{"content":"x"} trailing`, strings.Repeat("x", maxResponseBytes+1)} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(body))
		}))
		_, err := (Client{Endpoint: s.URL}).Complete(context.Background(), CompletionRequest{Prompt: "x", Temperature: 1, NPredict: 1})
		s.Close()
		if err == nil {
			t.Error("expected error")
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"content":"x"} {"extra":1}`)) }))
	_, err := (Client{Endpoint: s.URL}).Complete(context.Background(), CompletionRequest{Prompt: "x", Temperature: 1, NPredict: 1})
	s.Close()
	if err == nil {
		t.Error("trailing JSON accepted")
	}
}

func TestCompletionCancellation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (Client{Endpoint: s.URL}).Complete(ctx, CompletionRequest{Prompt: "x", Temperature: 1, NPredict: 1})
	if err == nil {
		t.Fatal("expected cancellation")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
