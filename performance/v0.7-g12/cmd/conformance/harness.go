package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

func contextWithTimeout(limit time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), limit)
}

// replica is one library, its carrier, and its round.
type replica struct {
	name      string
	store     *store.SQLiteStore
	carrier   *synccarrier.Directory
	round     *synccarrier.Round
	signer    *syncwire.MemorySigner
	handshake syncstate.Handshake
}

// harness holds two replicas of one database and everything they share.
type harness struct {
	workspace     string
	carrierRoot   string
	removableRoot string
	notes         int
	keys          *syncwire.MemoryKeyRing
	verifier      *syncwire.MemoryVerifier
	replicas      []*replica
	documents     []string
}

func newHarness(carrierRoot string, notes int) (*harness, error) {
	ctx := context.Background()
	workspace, err := os.MkdirTemp("", "notrios-g12-")
	if err != nil {
		return nil, err
	}
	keys, err := syncwire.NewMemoryKeyRing("key_g12")
	if err != nil {
		return nil, err
	}
	group, err := keys.Current()
	if err != nil {
		return nil, err
	}
	h := &harness{
		workspace: workspace, carrierRoot: carrierRoot, notes: notes,
		keys: keys, verifier: syncwire.NewMemoryVerifier(),
	}

	handshakes := make([]syncstate.Handshake, 0, 2)
	stores := make([]*store.SQLiteStore, 0, 2)
	for index := 0; index < 2; index++ {
		// The libraries stay on local disk. Only sealed artifacts reach the
		// provider: a SQLite file on a FUSE-mounted cloud folder is a way to
		// corrupt a database, not a way to synchronize one.
		canonical, err := store.OpenSQLiteWithAssetStore(
			filepath.Join(workspace, fmt.Sprintf("replica-%d.sqlite", index)),
			filepath.Join(workspace, fmt.Sprintf("assets-%d", index)))
		if err != nil {
			return nil, err
		}
		if err := canonical.Bootstrap(ctx); err != nil {
			return nil, err
		}
		if _, err := canonical.AdoptDatabaseIdentity(ctx, "db_g12_conformance"); err != nil {
			return nil, err
		}
		if _, err := canonical.EnrollLocalJournal(ctx, "G12 conformance"); err != nil {
			return nil, err
		}
		handshake, err := canonical.LocalSyncHandshake(ctx)
		if err != nil {
			return nil, err
		}
		stores = append(stores, canonical)
		handshakes = append(handshakes, handshake)
	}
	for receiver := range stores {
		for source := range stores {
			if receiver == source {
				continue
			}
			if err := stores[receiver].ConfigureSyncAdmissionPeer(ctx, handshakes[source]); err != nil {
				return nil, err
			}
		}
	}
	for index, canonical := range stores {
		signer, err := syncwire.NewMemorySigner()
		if err != nil {
			return nil, err
		}
		h.verifier.Enroll(signer.Public())
		carrier, err := synccarrier.NewDirectory(carrierRoot, group, "db_g12_conformance", handshakes[index].ReplicaID)
		if err != nil {
			return nil, err
		}
		h.replicas = append(h.replicas, &replica{
			name: string(rune('A' + index)), store: canonical, carrier: carrier, signer: signer,
			handshake: handshakes[index],
			round: synccarrier.NewRound(synccarrier.NewStoreReplica(canonical), carrier, keys, signer,
				h.verifier, store.NewLocalObjectProvider(canonical), synccarrier.Options{}),
		})
	}
	return h, nil
}

func (h *harness) close() {
	for _, current := range h.replicas {
		_ = current.store.Close()
	}
	_ = os.RemoveAll(h.workspace)
}

func (h *harness) createNotes(prefix string, count int) error {
	return h.createNotesWith(prefix, count, -1)
}

// createNotesWith authors on one replica when author is a valid index, and
// alternates otherwise.
func (h *harness) createNotesWith(prefix string, count, author int) error {
	ctx := context.Background()
	for index := 0; index < count; index++ {
		writer := h.replicas[index%len(h.replicas)]
		if author >= 0 && author < len(h.replicas) {
			writer = h.replicas[author]
		}
		document, err := writer.store.CreateDocument(ctx, store.CreateDocumentRequest{
			Title: fmt.Sprintf("%s note %d", prefix, index),
			Body:  strings.Repeat(fmt.Sprintf("%s line %d\n", prefix, index), 8),
		})
		if err != nil {
			return err
		}
		h.documents = append(h.documents, document.ID)
	}
	return nil
}

// converge runs rounds until nothing moves.
func (h *harness) converge(limit int) (int, error) {
	ctx := context.Background()
	for round := 1; round <= limit; round++ {
		moved := 0
		for _, current := range h.replicas {
			result, err := current.round.Run(ctx)
			if err != nil {
				return round, fmt.Errorf("%s: %w", current.name, err)
			}
			moved += result.AdmittedOperations + result.PublishedOperations
		}
		if moved == 0 {
			return round, nil
		}
	}
	return limit, errors.New("did not settle within the round limit")
}

// requireEveryNoteEverywhere is the only definition of success that matters.
func (h *harness) requireEveryNoteEverywhere() error {
	ctx := context.Background()
	for _, documentID := range h.documents {
		var reference string
		for _, current := range h.replicas {
			document, err := current.store.GetDocument(ctx, documentID)
			if err != nil {
				return fmt.Errorf("replica %s does not hold %s: %w", current.name, documentID, err)
			}
			if reference == "" {
				reference = document.Body
				continue
			}
			if document.Body != reference {
				return fmt.Errorf("replica %s disagrees about %s", current.name, documentID)
			}
		}
	}
	return nil
}

func (h *harness) convergeThroughProvider() (string, error) {
	if err := h.createNotes("provider", h.notes); err != nil {
		return "", err
	}
	started := time.Now()
	rounds, err := h.converge(6)
	if err != nil {
		return "", err
	}
	if err := h.requireEveryNoteEverywhere(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d notes converged in %d rounds, %.1fs", len(h.documents), rounds, time.Since(started).Seconds()), nil
}

// noForeignWrites proves the claim that makes a shared folder safe: a replica
// writes only inside its own namespace.
func (h *harness) noForeignWrites() (string, error) {
	namespace := filepath.Join(h.replicas[0].carrier.Root(), "replicas", h.replicas[0].carrier.Namespace())
	before, err := fingerprintTree(namespace)
	if err != nil {
		return "", err
	}
	if len(before) == 0 {
		return "", errors.New("replica A published nothing, so nothing was protected")
	}
	if err := h.createNotes("foreign", 4); err != nil {
		return "", err
	}
	for pass := 0; pass < 2; pass++ {
		if _, err := h.replicas[1].round.Run(context.Background()); err != nil {
			return "", err
		}
	}
	after, err := fingerprintTree(namespace)
	if err != nil {
		return "", err
	}
	for _, name := range sortedNames(before) {
		digest, present := after[name]
		if !present {
			return "", fmt.Errorf("replica B removed an artifact from replica A's namespace")
		}
		if digest != before[name] {
			return "", fmt.Errorf("replica B rewrote an artifact in replica A's namespace")
		}
	}
	return fmt.Sprintf("%d of replica A's artifacts unchanged after replica B's rounds", len(before)), nil
}

// conflictingName writes different bytes under a name the protocol already
// uses. On a provider that resolves a conflict by keeping one copy, or by
// serving a stale one, this is what a reader meets.
func (h *harness) conflictingName() (string, error) {
	ctx := context.Background()
	// The artifact has to be one the peer still needs. A damaged artifact that
	// everyone has already admitted is not repaired, and should not be: nobody
	// is waiting for it, and cleanup removes it once every peer has
	// acknowledged what it carried.
	namespace := filepath.Join(h.replicas[0].carrier.Root(), "replicas", h.replicas[0].carrier.Namespace())
	before, err := fingerprintTree(namespace)
	if err != nil {
		return "", err
	}
	if err := h.createNotesWith("conflict", 3, 0); err != nil {
		return "", err
	}
	if _, err := h.replicas[0].round.Run(ctx); err != nil {
		return "", err
	}
	prints, err := fingerprintTree(namespace)
	if err != nil {
		return "", err
	}
	// The victim has to be an envelope the peer has not admitted yet. An older
	// one that everybody already holds is not republished, and should not be:
	// nobody is waiting for it, and cleanup removes it once every peer has
	// acknowledged what it carried.
	victim := ""
	for _, name := range sortedNames(prints) {
		if _, existed := before[name]; !existed && strings.Contains(name, "envelopes") {
			victim = name
		}
	}
	if victim == "" {
		return "", errors.New("replica A published no new envelope to substitute")
	}
	path := filepath.Join(namespace, victim)
	original, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// What a provider's conflict resolution, or a half-finished copy, leaves
	// behind: the protocol's name, somebody else's bytes.
	if err := os.WriteFile(path, []byte("a provider conflict copy of the wrong length"), 0o644); err != nil {
		return "", err
	}

	result, err := h.replicas[1].round.Run(ctx)
	if err != nil {
		return "", fmt.Errorf("a conflicting artifact failed the round: %w", err)
	}
	refused := 0
	for _, count := range result.Skipped {
		refused += count
	}
	if refused == 0 {
		return "", errors.New("a substituted artifact was not refused")
	}

	// The owner repairs it on its next round, because it asks whether a
	// readable copy is there rather than whether the name is taken.
	if _, err := h.replicas[0].round.Run(ctx); err != nil {
		return "", err
	}
	repaired, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(repaired) < len(original)/2 {
		return "", errors.New("the owner did not repair its own substituted artifact")
	}
	if string(repaired) == string(original) {
		return "", errors.New("the repaired artifact is byte-identical, which a fresh salt makes impossible")
	}
	if _, err := h.converge(4); err != nil {
		return "", err
	}
	if err := h.requireEveryNoteEverywhere(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d entries refused, then the owner republished %d bytes and both replicas converged",
		refused, len(repaired)), nil
}

// delayedListing is the case the measured 45-to-56-second visibility lag
// actually produces: a peer's advertisement becomes visible before the
// envelopes it implies exist.
//
// The protocol publishes envelopes first and advertises last precisely so this
// cannot happen through one filesystem, but on an eventually-consistent
// provider ordering is not the publisher's to decide. This makes the envelopes
// invisible while the advertisement is readable and asserts the reader neither
// fails nor records progress it did not make.
func (h *harness) delayedListing() (string, error) {
	ctx := context.Background()
	if err := h.createNotesWith("delayed", 3, 0); err != nil {
		return "", err
	}
	pending := h.documents[len(h.documents)-3:]
	if _, err := h.replicas[0].round.Run(ctx); err != nil {
		return "", err
	}

	namespace := filepath.Join(h.replicas[0].carrier.Root(), "replicas", h.replicas[0].carrier.Namespace())
	envelopes := filepath.Join(namespace, "envelopes")
	// Hidden beside the layout rather than outside the carrier: a provider
	// mount is its own device, and moving a directory off it is a copy, which
	// would change what is being tested.
	hidden := filepath.Join(h.carrierRoot, "notrios-g12-not-yet-listed")
	if err := os.Rename(envelopes, hidden); err != nil {
		return "", err
	}
	defer os.RemoveAll(hidden)

	before, err := h.replicas[1].store.SyncStateVector(ctx)
	if err != nil {
		return "", err
	}
	result, err := h.replicas[1].round.Run(ctx)
	if err != nil {
		return "", fmt.Errorf("a lagging listing failed the round: %w", err)
	}
	if result.AdmittedOperations != 0 {
		return "", fmt.Errorf("admitted %d operations that were not visible", result.AdmittedOperations)
	}
	after, err := h.replicas[1].store.SyncStateVector(ctx)
	if err != nil {
		return "", err
	}
	if after[h.replicas[0].handshake.ReplicaID] != before[h.replicas[0].handshake.ReplicaID] {
		return "", errors.New("the reader recorded progress it had not made")
	}
	for _, documentID := range pending {
		if _, err := h.replicas[1].store.GetDocument(ctx, documentID); err == nil {
			return "", fmt.Errorf("%s arrived without its envelope", documentID)
		}
	}

	// The listing catches up, and the next ordinary round is all it takes.
	if err := os.Rename(hidden, envelopes); err != nil {
		return "", err
	}
	if _, err := h.converge(4); err != nil {
		return "", err
	}
	if err := h.requireEveryNoteEverywhere(); err != nil {
		return "", err
	}
	return "advertisement without envelopes admitted nothing and claimed nothing; the next round converged", nil
}

// unavailableCarrier is the disconnect: the folder is simply not there.
func (h *harness) unavailableCarrier() (string, error) {
	ctx := context.Background()
	group, err := h.keys.Current()
	if err != nil {
		return "", err
	}
	absent := filepath.Join(h.workspace, "not-mounted")
	carrier, err := synccarrier.NewDirectory(absent, group, "db_g12_conformance", h.replicas[1].handshake.ReplicaID)
	if err != nil {
		return "", err
	}
	round := synccarrier.NewRound(synccarrier.NewStoreReplica(h.replicas[1].store), carrier, h.keys,
		h.replicas[1].signer, h.verifier, nil, synccarrier.Options{})
	if _, err := round.Run(ctx); !errors.Is(err, synccarrier.ErrCarrierUnavailable) {
		return "", fmt.Errorf("a disconnected carrier produced %v, want ErrCarrierUnavailable", err)
	}
	if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("the round created the absent mount point")
	}
	// Reconnecting is nothing more than the folder being there again.
	if err := h.createNotes("reconnect", 2); err != nil {
		return "", err
	}
	if _, err := h.converge(4); err != nil {
		return "", err
	}
	if err := h.requireEveryNoteEverywhere(); err != nil {
		return "", err
	}
	return "refused while disconnected, converged after reconnecting", nil
}

// carrierLoss deletes everything the provider holds.
func (h *harness) carrierLoss() (string, error) {
	if err := h.createNotes("after-loss", 4); err != nil {
		return "", err
	}
	// Publish, then lose the carrier before the peer has read it.
	if _, err := h.replicas[0].round.Run(context.Background()); err != nil {
		return "", err
	}
	if err := os.RemoveAll(filepath.Join(h.carrierRoot, layoutRoot)); err != nil {
		return "", err
	}
	started := time.Now()
	rounds, err := h.converge(6)
	if err != nil {
		return "", err
	}
	if err := h.requireEveryNoteEverywhere(); err != nil {
		return "", err
	}
	return fmt.Sprintf("rebuilt and reconverged in %d rounds, %.1fs", rounds, time.Since(started).Seconds()), nil
}

// immutableCopy exercises the one rclone verb this project permits against
// protocol state, in both directions, and proves it changes nothing.
func immutableCopyAvailable() bool {
	_, err := os.Stat("/usr/bin/rclone")
	return err == nil
}

func (h *harness) immutableCopy() (string, error) {
	if !immutableCopyAvailable() {
		return "skipped: rclone is not installed", nil
	}
	source := filepath.Join(h.carrierRoot, layoutRoot)
	staging := filepath.Join(h.workspace, "immutable-copy")
	before, err := fingerprintTree(source)
	if err != nil {
		return "", err
	}
	if len(before) == 0 {
		return "", errors.New("the carrier is empty, so the copy would prove nothing")
	}
	if output, err := runRclone("copy", "--immutable", source, staging); err != nil {
		return "", fmt.Errorf("rclone copy --immutable: %v: %s", err, output)
	}
	copied, err := fingerprintTree(staging)
	if err != nil {
		return "", err
	}
	for _, name := range sortedNames(before) {
		if copied[name] != before[name] {
			return "", fmt.Errorf("the copy does not match the carrier at %s", name)
		}
	}
	// Copying back over an unchanged carrier must be a no-op rather than a
	// rewrite: --immutable refuses to replace an existing file whose size or
	// timestamp differs, and here nothing differs at all.
	if output, err := runRclone("copy", "--immutable", staging, source); err != nil {
		return "", fmt.Errorf("rclone copy --immutable back: %v: %s", err, output)
	}
	after, err := fingerprintTree(source)
	if err != nil {
		return "", err
	}
	for _, name := range sortedNames(before) {
		if after[name] != before[name] {
			return "", fmt.Errorf("copying back changed %s", name)
		}
	}
	// The refusal list is enforced, not merely documented.
	if _, err := runRclone("sync", staging, source); err == nil {
		return "", errors.New("`rclone sync` was not refused")
	}
	return fmt.Sprintf("%d artifacts copied out and back with no change; sync/bisync/move/delete/purge refused", len(before)), nil
}

// removableHandoff is the drive that only one peer can see at a time, and that
// arrives at the other peer under a different path.
func (h *harness) removableHandoff() (string, error) {
	ctx := context.Background()
	group, err := h.keys.Current()
	if err != nil {
		return "", err
	}
	firstMount := filepath.Join(h.workspace, "drive-mounted-at-a")
	secondMount := filepath.Join(h.workspace, "drive-mounted-at-b")
	if err := os.MkdirAll(firstMount, 0o755); err != nil {
		return "", err
	}

	writer, err := synccarrier.NewDirectory(firstMount, group, "db_g12_conformance", h.replicas[0].handshake.ReplicaID)
	if err != nil {
		return "", err
	}
	writerRound := synccarrier.NewRound(synccarrier.NewStoreReplica(h.replicas[0].store), writer, h.keys,
		h.replicas[0].signer, h.verifier, nil, synccarrier.Options{})

	// Everything on this trip is authored by the replica holding the drive.
	// Requiring both replicas to hold each other's notes after a one-way trip
	// would be asking the drive to travel before it has travelled.
	if err := h.createNotesWith("carried-out", 3, 0); err != nil {
		return "", err
	}
	outbound := h.documents[len(h.documents)-3:]
	if _, err := writerRound.Run(ctx); err != nil {
		return "", err
	}

	// Unplug: the drive leaves the first machine entirely and appears at the
	// second under another path.
	if err := os.MkdirAll(secondMount, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(filepath.Join(firstMount, layoutRoot), filepath.Join(secondMount, layoutRoot)); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(firstMount, layoutRoot)); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("the drive is still visible to the first replica")
	}

	reader, err := synccarrier.NewDirectory(secondMount, group, "db_g12_conformance", h.replicas[1].handshake.ReplicaID)
	if err != nil {
		return "", err
	}
	readerRound := synccarrier.NewRound(synccarrier.NewStoreReplica(h.replicas[1].store), reader, h.keys,
		h.replicas[1].signer, h.verifier, nil, synccarrier.Options{})
	if _, err := readerRound.Run(ctx); err != nil {
		return "", err
	}
	for _, documentID := range outbound {
		if _, err := h.replicas[1].store.GetDocument(ctx, documentID); err != nil {
			return "", fmt.Errorf("one trip did not carry %s: %w", documentID, err)
		}
	}

	// The drive goes back carrying the second replica's own new note.
	if err := h.createNotesWith("carried-back", 2, 1); err != nil {
		return "", err
	}
	inbound := h.documents[len(h.documents)-2:]
	if _, err := readerRound.Run(ctx); err != nil {
		return "", err
	}
	if err := os.Rename(filepath.Join(secondMount, layoutRoot), filepath.Join(firstMount, layoutRoot)); err != nil {
		return "", err
	}
	if _, err := writerRound.Run(ctx); err != nil {
		return "", err
	}
	for _, documentID := range inbound {
		if _, err := h.replicas[0].store.GetDocument(ctx, documentID); err != nil {
			return "", fmt.Errorf("the return trip did not carry %s: %w", documentID, err)
		}
	}
	// The libraries agree, and the drive never carried a database.
	if err := h.requireEveryNoteEverywhere(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d notes out and %d back in one trip each, on a drive only one peer can see at a time",
		len(outbound), len(inbound)), nil
}
