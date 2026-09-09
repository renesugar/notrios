// Package syncrest carries G11's protocol artifacts over G13's authenticated
// surface.
//
// It implements `synccarrier.Carrier`, which is the whole design: the exchange
// was written against that interface, so a REST peer and a shared folder are
// the same protocol running over different couriers. The plan asked for
// "REST/directory golden transcript parity"; this is parity by construction
// rather than by two implementations somebody has to keep in step.
package syncrest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncwire"
)

// Carrier is a peer's sync surface, used as a carrier.
type Carrier struct {
	client    *syncauth.Client
	namespace string
}

// New binds a signing client to the namespace this replica writes under.
//
// The namespace is computed locally from the same keyed blind the folder layout
// uses, so a peer and a folder address the same replica identically — and the
// server derives it again from the authenticated principal rather than
// believing this one.
func New(client *syncauth.Client, group syncwire.GroupKey, replicaID string) *Carrier {
	return &Carrier{client: client, namespace: syncwire.CarrierName(group, "replica", replicaID)}
}

// Namespace implements synccarrier.Carrier.
func (c *Carrier) Namespace() string { return c.namespace }

// Initialize implements synccarrier.Carrier. A REST peer prepares its own
// storage, so this only checks that the peer is reachable and willing.
func (c *Carrier) Initialize(ctx context.Context) error {
	status, _, err := c.client.Do(ctx, http.MethodGet, "/api/v1/sync/carrier/namespaces", nil)
	if err != nil {
		return fmt.Errorf("%w: %v", synccarrier.ErrCarrierUnavailable, err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("%w: peer answered %d", synccarrier.ErrCarrierUnavailable, status)
	}
	return nil
}

// Namespaces implements synccarrier.Carrier.
func (c *Carrier) Namespaces(ctx context.Context) ([]string, error) {
	status, body, err := c.client.Do(ctx, http.MethodGet, "/api/v1/sync/carrier/namespaces", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", synccarrier.ErrCarrierUnavailable, err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: peer answered %d", synccarrier.ErrCarrierUnavailable, status)
	}
	var answer struct {
		Namespaces []string `json:"namespaces"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("%w: %v", synccarrier.ErrCarrierUnavailable, err)
	}
	return answer.Namespaces, nil
}

// List implements synccarrier.Carrier.
func (c *Carrier) List(ctx context.Context, namespace string, class synccarrier.Class) ([]string, error) {
	status, body, err := c.client.Do(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/sync/carrier/%s/%s", namespace, class), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", synccarrier.ErrCarrierUnavailable, err)
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: peer answered %d", synccarrier.ErrCarrierUnavailable, status)
	}
	var answer struct {
		Names []string `json:"names"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("%w: %v", synccarrier.ErrCarrierUnavailable, err)
	}
	return answer.Names, nil
}

// Read implements synccarrier.Carrier.
//
// A missing or refused artifact is ErrUnreadable rather than a failure: the
// round treats a carrier that cannot answer as an ordinary skip, and a peer is
// no more entitled to stop a round than a folder is.
func (c *Carrier) Read(ctx context.Context, namespace string, class synccarrier.Class, name string) ([]byte, error) {
	status, body, err := c.client.Do(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/sync/carrier/%s/%s/%s", namespace, class, name), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", synccarrier.ErrUnreadable, err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: peer answered %d", synccarrier.ErrUnreadable, status)
	}
	return body, nil
}

// Publish implements synccarrier.Carrier. The path carries no namespace: the
// peer decides that from who is asking.
func (c *Carrier) Publish(ctx context.Context, class synccarrier.Class, name string, artifact []byte) (string, error) {
	if name == "" {
		return "", errors.New("syncrest: a published artifact needs its protocol name")
	}
	status, body, err := c.client.Do(ctx, http.MethodPut,
		fmt.Sprintf("/api/v1/sync/carrier/%s/%s", class, name), artifact)
	if err != nil {
		return "", fmt.Errorf("%w: %v", synccarrier.ErrCarrierUnavailable, err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("%w: peer answered %d: %s", synccarrier.ErrCarrierUnavailable,
			status, strings.TrimSpace(string(body)))
	}
	return name, nil
}

// Remove implements synccarrier.Carrier.
func (c *Carrier) Remove(ctx context.Context, class synccarrier.Class, name string) error {
	status, _, err := c.client.Do(ctx, http.MethodDelete,
		fmt.Sprintf("/api/v1/sync/carrier/%s/%s", class, name), nil)
	if err != nil {
		return fmt.Errorf("%w: %v", synccarrier.ErrCarrierUnavailable, err)
	}
	if status != http.StatusOK && status != http.StatusNotFound {
		return fmt.Errorf("%w: peer answered %d", synccarrier.ErrCarrierUnavailable, status)
	}
	return nil
}
