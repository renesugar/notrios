package synccarrier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/renesugar/notrios/internal/syncassets"
	"github.com/renesugar/notrios/internal/syncwire"
)

// ErrObjectUnavailable reports that no peer namespace on this carrier holds the
// requested object bytes yet. It is the ordinary answer, not a failure: a
// replica asks by publishing a request and finds the answer on a later round.
var ErrObjectUnavailable = errors.New("no peer on this carrier has published these object bytes")

func encodeManifest(manifest syncassets.Manifest) ([]byte, error) {
	return json.Marshal(manifest)
}

func decodeManifest(plaintext []byte) (syncassets.Manifest, error) {
	if len(plaintext) > MaxControlBytes {
		return syncassets.Manifest{}, fmt.Errorf("%w: %d manifest bytes", ErrControl, len(plaintext))
	}
	var manifest syncassets.Manifest
	if err := json.Unmarshal(plaintext, &manifest); err != nil {
		return syncassets.Manifest{}, fmt.Errorf("%w: %v", ErrControl, err)
	}
	return manifest, nil
}

// Provider fetches resource bytes from a carrier. It implements the
// ObjectProvider G8 defined and left for this slice to supply, so lazy
// materialization runs over a shared folder with no change to the verification
// it already does.
//
// Nothing here verifies content: the manifest digest, each chunk hash, the
// whole-object hash, the length, and the sniffed type are all checked by the
// store after this returns. A transport that verified content would be a second
// place to get it wrong.
type Provider struct {
	carrier  Carrier
	keys     syncwire.KeyRing
	verifier syncwire.Verifier
	limits   syncwire.Limits
}

// NewProvider builds a carrier-backed object provider.
func NewProvider(carrier Carrier, keys syncwire.KeyRing, verifier syncwire.Verifier, limits syncwire.Limits) *Provider {
	return &Provider{carrier: carrier, keys: keys, verifier: verifier, limits: limits}
}

// FetchManifest returns an object's chunk manifest from whichever peer
// published it.
func (p *Provider) FetchManifest(ctx context.Context, blobSHA256 string) (syncassets.Manifest, error) {
	plaintext, err := p.fetch(ctx, blobSHA256, manifestOrdinal)
	if err != nil {
		return syncassets.Manifest{}, err
	}
	return decodeManifest(plaintext)
}

// FetchChunk returns one transfer segment.
func (p *Provider) FetchChunk(ctx context.Context, blobSHA256 string, ordinal int) ([]byte, error) {
	return p.fetch(ctx, blobSHA256, ordinal)
}

// fetch looks for one object part in every namespace on the carrier.
//
// Two peers may both hold an object, and their sealed copies differ byte for
// byte because every seal draws a fresh salt. That is why a copy that fails to
// open is skipped rather than treated as the answer: the next namespace may
// hold an intact one, and the content check downstream is unchanged either way.
func (p *Provider) fetch(ctx context.Context, blobSHA256 string, ordinal int) ([]byte, error) {
	group, err := p.keys.Current()
	if err != nil {
		return nil, err
	}
	name := ObjectName(group, blobSHA256, ordinal)
	namespaces, err := p.carrier.Namespaces(ctx)
	if err != nil {
		return nil, err
	}
	for _, namespace := range namespaces {
		if namespace == p.carrier.Namespace() {
			continue
		}
		artifact, err := p.carrier.Read(ctx, namespace, ClassObject, name)
		if err != nil {
			continue
		}
		_, plaintext, err := syncwire.Open(p.keys, p.verifier, artifact, p.limits)
		if err != nil {
			continue
		}
		return plaintext, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrObjectUnavailable, shortAddress(blobSHA256, ordinal))
}

// shortAddress names an object in an error without printing a full content
// hash into a log a user may share.
func shortAddress(blobSHA256 string, ordinal int) string {
	prefix := blobSHA256
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	if ordinal < 0 {
		return prefix + "#manifest"
	}
	return fmt.Sprintf("%s#%d", prefix, ordinal)
}
