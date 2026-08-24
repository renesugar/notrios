package syncauth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Response bounds keep an authenticated peer from choosing an allocation.
// Ordinary JSON/artifact replies retain the smaller ceiling. A signed Range
// request may return one G14 backup-download chunk, still independently
// bounded and verified by its declared Content-Range and final digest.
const (
	MaxResponseBytes      = 8 << 20
	MaxRangeResponseBytes = 16 << 20
)

// BackupCreationTimeout is the single exceptional authenticated-request
// deadline. Producing a full-corpus snapshot is synchronous in protocol 1.0
// and must finish inside G14's two-hour stage ceiling.
const BackupCreationTimeout = 2 * time.Hour

// Client signs requests to a peer's sync surface.
//
// It holds a private key and therefore never logs a request, a header, or a
// response body. What it returns is what the caller asked for and the status
// that came with it.
type Client struct {
	BaseURL     string
	DatabaseID  string
	ReplicaID   string
	SignerKeyID string
	Private     ed25519.PrivateKey
	HTTP        *http.Client
	Now         func() time.Time
}

// ErrInsecureTransport reports an attempt to authenticate to a non-loopback
// peer over plaintext HTTP.
var ErrInsecureTransport = errors.New("refusing to authenticate over plaintext to a non-loopback peer")

// Do performs one signed request and returns the status and body.
//
// The transport check is here rather than only on the server because both ends
// have to hold the line: a client that happily signs over plaintext to a remote
// host is the thing that makes an operator's misconfiguration invisible.
func (c *Client) Do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	return c.do(ctx, method, path, body, "", 0, MaxResponseBytes)
}

// DoWithin performs one signed request with a bounded operation-specific HTTP
// deadline. It exists for snapshot creation, whose authenticated response
// cannot begin until a potentially multi-gigabyte Online Backup has finished;
// ordinary sync requests retain the short default deadline.
func (c *Client) DoWithin(ctx context.Context, method, path string, body []byte, timeout time.Duration) (int, []byte, error) {
	if timeout <= 0 {
		return 0, nil, fmt.Errorf("%w: request timeout must be positive", ErrMalformed)
	}
	return c.do(ctx, method, path, body, "", timeout, MaxResponseBytes)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, byteRange string, requestTimeout time.Duration, responseLimit int64) (int, []byte, error) {
	if c.Private == nil || c.ReplicaID == "" || c.DatabaseID == "" {
		return 0, nil, fmt.Errorf("%w: a client needs its identity and key", ErrMalformed)
	}
	target, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return 0, nil, err
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return 0, nil, err
	}
	if parsed.Scheme != "https" && !isLoopbackHost(parsed.Hostname()) {
		return 0, nil, fmt.Errorf("%w: %s", ErrInsecureTransport, parsed.Host)
	}
	nonce := make([]byte, NonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return 0, nil, err
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	request := Request{
		Method: method, Path: parsed.Path, DatabaseID: c.DatabaseID, ReplicaID: c.ReplicaID,
		Timestamp: now().UTC(), Nonce: hex.EncodeToString(nonce), BodySHA256: BodyDigest(body),
	}
	header, err := Sign(c.Private, c.SignerKeyID, request)
	if err != nil {
		return 0, nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	httpRequest.Header.Set("Authorization", header)
	if len(body) > 0 {
		httpRequest.Header.Set("Content-Type", "application/octet-stream")
	}
	if byteRange != "" {
		httpRequest.Header.Set("Range", byteRange)
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if requestTimeout > 0 {
		clone := *client
		clone.Timeout = requestTimeout
		client = &clone
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(response.Body, responseLimit))
	if err != nil {
		return response.StatusCode, nil, err
	}
	return response.StatusCode, answer, nil
}

// DoRange performs one signed request asking for a byte range.
//
// The range header is deliberately *not* part of the signature: it names which
// bytes of an artifact the caller wants, not what the artifact is, and a
// resumed download would otherwise need a fresh signature per attempt whose
// only difference was an offset. What the signature covers is the object being
// requested; what the peer returns is verified against the hash it declared.
func (c *Client) DoRange(ctx context.Context, method, path, byteRange string) (int, []byte, error) {
	return c.do(ctx, method, path, nil, byteRange, 0, MaxRangeResponseBytes)
}

// Pair performs the unauthenticated pairing request, which carries the code in
// a header rather than the body so it is never part of the signed or logged
// payload a proxy might retain.
func Pair(ctx context.Context, client *http.Client, baseURL, code string, request PairingRequest) (int, []byte, error) {
	target, err := url.JoinPath(baseURL, "/api/v1/sync/pair")
	if err != nil {
		return 0, nil, err
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return 0, nil, err
	}
	if parsed.Scheme != "https" && !isLoopbackHost(parsed.Hostname()) {
		return 0, nil, fmt.Errorf("%w: %s", ErrInsecureTransport, parsed.Host)
	}
	body, err := encodeJSON(request)
	if err != nil {
		return 0, nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("X-Notrios-Pairing-Code", code)
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(response.Body, MaxPairingBodyBytes))
	if err != nil {
		return response.StatusCode, nil, err
	}
	return response.StatusCode, answer, nil
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "127.")
}

func encodeJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}
