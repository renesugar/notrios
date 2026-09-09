// Package doccheck contains the small, local-only client used by doccheck.
package doccheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxResponseBytes = 2 << 20
	maxPromptBytes   = 1 << 20
	maxGrammarBytes  = 64 << 10
	maxStopBytes     = 4096
	maxStopCount     = 16
)

// Client talks to a llama.cpp server on the local machine. Endpoint must be
// an http URL rooted at a loopback host. HTTPClient is optional.
type Client struct {
	Endpoint   string
	HTTPClient *http.Client
	Timeout    time.Duration
}

type CompletionRequest struct {
	Prompt      string
	Temperature float64
	NPredict    int
	Stop        []string
	Grammar     string
}

type CompletionResult struct {
	Content         string  `json:"content"`
	Model           string  `json:"model"`
	TokensPredicted int     `json:"tokens_predicted"`
	TokensEvaluated int     `json:"tokens_evaluated"`
	Stop            bool    `json:"stop"`
	StopType        string  `json:"stop_type"`
	Timings         Timings `json:"timings"`
}

type Timings struct {
	PromptMilliseconds    float64 `json:"prompt_ms"`
	PredictedMilliseconds float64 `json:"predicted_ms"`
}

type healthResponse struct {
	Status string `json:"status"`
}

func (c Client) baseURL() (string, error) {
	u, err := url.Parse(strings.TrimSpace(c.Endpoint))
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Host == "" {
		return "", errors.New("endpoint must be a plain http loopback URL")
	}
	host := u.Hostname()
	if host != "127.0.0.1" && host != "::1" && host != "localhost" {
		return "", errors.New("endpoint host is not loopback")
	}
	if u.Path != "" && u.Path != "/" || u.RawPath != "" {
		return "", errors.New("endpoint path must be empty or /")
	}
	if strings.Contains(u.Host, "%") || strings.Contains(u.Host, "[") && host != "::1" {
		return "", errors.New("endpoint host is invalid")
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return "", errors.New("endpoint port is invalid")
		}
	}
	port := u.Port()
	if port == "" {
		if host == "::1" {
			return "http://[::1]", nil
		}
		return "http://" + host, nil
	}
	return "http://" + net.JoinHostPort(host, port), nil
}

func (c Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	base, err := c.baseURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxResponseBytes {
		return nil, errors.New("response exceeds 2 MiB limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(b))
		if len(msg) > 512 {
			msg = msg[:512]
		}
		return nil, fmt.Errorf("llama.cpp returned HTTP %s: %s", resp.Status, msg)
	}
	return b, nil
}

func decodeStrict(b []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("response contains trailing JSON")
		}
		return err
	}
	return nil
}

func (c Client) Health(ctx context.Context) error {
	b, err := c.do(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return err
	}
	var h healthResponse
	if err := decodeStrict(b, &h); err != nil {
		return fmt.Errorf("invalid health response: %w", err)
	}
	if h.Status != "ok" {
		return fmt.Errorf("llama.cpp health status is %q", h.Status)
	}
	return nil
}

func (r CompletionRequest) validate() error {
	if r.Prompt == "" || len([]byte(r.Prompt)) > maxPromptBytes {
		return errors.New("prompt must be nonempty and at most 1 MiB")
	}
	if r.Temperature < 0 || r.Temperature > 2 {
		return errors.New("temperature must be between 0 and 2")
	}
	if r.NPredict < 1 || r.NPredict > 4096 {
		return errors.New("n_predict must be between 1 and 4096")
	}
	if len(r.Stop) > maxStopCount {
		return errors.New("stop may contain at most 16 strings")
	}
	for _, s := range r.Stop {
		if s == "" || len([]byte(s)) > maxStopBytes {
			return errors.New("stop strings must be nonempty and at most 4096 bytes")
		}
	}
	if len([]byte(r.Grammar)) > maxGrammarBytes {
		return errors.New("grammar must be at most 64 KiB")
	}
	return nil
}

func (c Client) Complete(ctx context.Context, r CompletionRequest) (CompletionResult, error) {
	var out CompletionResult
	if err := r.validate(); err != nil {
		return out, err
	}
	payload := struct {
		Prompt      string   `json:"prompt"`
		Temperature float64  `json:"temperature"`
		NPredict    int      `json:"n_predict"`
		Stop        []string `json:"stop,omitempty"`
		Grammar     string   `json:"grammar,omitempty"`
	}{r.Prompt, r.Temperature, r.NPredict, r.Stop, r.Grammar}
	b, err := json.Marshal(payload)
	if err != nil {
		return out, err
	}
	b, err = c.do(ctx, http.MethodPost, "/completion", b)
	if err != nil {
		return out, err
	}
	if err := decodeStrict(b, &out); err != nil {
		return out, fmt.Errorf("invalid completion response: %w", err)
	}
	return out, nil
}
