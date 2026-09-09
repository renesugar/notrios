// Command harness runs one independently resumable G14a benchmark phase.
// Benchmark workspaces contain generated or private working data and therefore
// belong outside the repository; only aggregate generated results are copied
// into performance/v0.7-g14a/calibration-results.json.
package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/renesugar/notrios/internal/archivev2"
	"github.com/renesugar/notrios/internal/importers/joplinraw"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncbackup"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncwire"
	bench "github.com/renesugar/notrios/performance/v0.7-g14a/harness"
)

type configuration struct {
	workspace string
	tier      int
	adapter   string
	phase     bench.Stage
	cache     string
}

func main() {
	workspace := flag.String("workspace", "", "benchmark workspace outside the repository")
	tier := flag.Int("tier", 0, "generated tier: 10000 or 100000")
	adapter := flag.String("adapter", "archive-v2-loose", "adapter id")
	phase := flag.String("phase", "", "one benchmark phase")
	cache := flag.String("cache-state", "interleaved-first", "interleaved-first, interleaved-repeat, or uncontrolled")
	describe := flag.Bool("describe-adapters", false, "print the adapter contract and exit")
	validate := flag.String("validate-results", "", "validate every JSON result below a workspace and exit")
	flag.Parse()
	if *describe {
		must(bench.ValidateAdapters(bench.Adapters()))
		writeJSON(bench.Adapters())
		return
	}
	if *validate != "" {
		must(validateResults(*validate))
		return
	}
	if *workspace == "" || !filepath.IsAbs(*workspace) {
		fail("-workspace must be an absolute path outside the repository")
	}
	cfg := configuration{workspace: filepath.Clean(*workspace), tier: *tier, adapter: *adapter, phase: bench.Stage(*phase), cache: *cache}
	if cfg.workspace == repositoryRoot() || strings.HasPrefix(cfg.workspace, repositoryRoot()+string(filepath.Separator)) {
		fail("benchmark workspace must be outside the repository")
	}
	if _, err := bench.AdapterByID(cfg.adapter); err != nil {
		fail(err.Error())
	}
	if err := bench.ValidateIdentity(cfg.tier, cfg.adapter, cfg.phase); err != nil {
		fail(err.Error())
	}
	runner := bench.Runner{Workspace: cfg.workspace, Tier: cfg.tier, Adapter: cfg.adapter, Phase: cfg.phase}
	result, resumed, err := runner.Run(func() (bench.Result, error) { return runPhase(cfg) })
	must(err)
	writeJSON(struct {
		Resumed bool         `json:"resumed_from_completed_phase"`
		Result  bench.Result `json:"result"`
	}{resumed, result})
}

func runPhase(cfg configuration) (bench.Result, error) {
	if cfg.adapter != "archive-v2-loose" && cfg.adapter != "archive-v2-pack" {
		return bench.Result{}, fmt.Errorf("adapter %s is defined for G14b but generated G14a execution supports archive-v2-loose and archive-v2-pack", cfg.adapter)
	}
	if err := ensureGeneratedSource(cfg); err != nil {
		return bench.Result{}, err
	}
	sourceBefore, err := bench.InspectTree(sourceRoot(cfg))
	if err != nil {
		return bench.Result{}, err
	}
	started := startMetrics()
	result := bench.Result{CacheState: cfg.cache, Generated: true, Environment: bench.DetectEnvironment(cfg.workspace), Source: sourceBefore}
	var phaseErr error
	switch cfg.phase {
	case bench.StageInventory:
		result.Output = sourceBefore
		result.Metrics.Items = sourceBefore.Files + sourceBefore.Directories + sourceBefore.Symlinks + sourceBefore.OtherEntries
		result.Metrics.InputBytes, result.Metrics.OutputBytes = sourceBefore.ApparentBytes, sourceBefore.ApparentBytes
	case bench.StageForeignImport:
		phaseErr = foreignImport(cfg, &result)
	case bench.StageSnapshotCreate:
		phaseErr = snapshotCreate(cfg, &result)
	case bench.StageSnapshotVerify:
		phaseErr = snapshotVerify(cfg, &result)
	case bench.StageTransportPrepare:
		phaseErr = transportPrepare(cfg, &result)
	case bench.StageTransportSeal:
		phaseErr = transportSeal(cfg, &result)
	case bench.StageSnapshotOpen:
		phaseErr = snapshotOpen(cfg, &result)
	case bench.StageSnapshotRestore:
		phaseErr = snapshotRestore(cfg, &result)
	case bench.StageIncrementalReplay:
		phaseErr = incrementalReplay(cfg, &result)
	default:
		phaseErr = fmt.Errorf("unsupported phase %s", cfg.phase)
	}
	if phaseErr != nil {
		return bench.Result{}, phaseErr
	}
	sourceAfter, err := bench.InspectTree(sourceRoot(cfg))
	if err != nil {
		return bench.Result{}, err
	}
	result.Assertions.SourceUnchanged = bench.SameInventory(sourceBefore, sourceAfter)
	result.Assertions.ArithmeticValid = result.Output.Directories == 0 || result.Output.Directories == result.Output.DirectoryHistogram.Total()
	finishMetrics(started, &result.Metrics)
	if result.Metrics.OutputEntries == 0 {
		result.Metrics.OutputEntries = result.Output.Files + result.Output.Directories + result.Output.Symlinks + result.Output.OtherEntries
	}
	if result.Metrics.MaxDirectoryItems == 0 {
		result.Metrics.MaxDirectoryItems = result.Output.MaximumDirectorySize
	}
	if result.Metrics.InputBytes > 0 && result.Metrics.OutputBytes > 0 {
		result.Metrics.CompressionRatio = float64(result.Metrics.InputBytes) / float64(result.Metrics.OutputBytes)
	}
	return result, nil
}

func foreignImport(cfg configuration, result *bench.Result) error {
	if err := os.MkdirAll(canonicalRoot(cfg), 0o755); err != nil {
		return err
	}
	st, err := openStore(canonicalRoot(cfg))
	if err != nil {
		return err
	}
	defer st.Close()
	report, err := joplinraw.Import(context.Background(), st, sourceRoot(cfg), joplinraw.Options{BatchSize: 500})
	if err != nil {
		return err
	}
	metrics, err := st.ImportMetrics(context.Background())
	if err != nil {
		return err
	}
	result.Metrics.Items = int64(report.ItemsSeen)
	result.Metrics.Resources = int64(report.ResourcesSeen)
	result.Metrics.InputBytes = result.Source.ApparentBytes
	result.Metrics.OutputBytes = metrics.DatabaseBytes
	result.Assertions.CanonicalFingerprint = aggregateCanonicalFingerprint(metrics)
	result.Output, err = bench.InspectTree(canonicalRoot(cfg))
	return err
}

func snapshotCreate(cfg configuration, result *bench.Result) error {
	st, err := openStore(canonicalRoot(cfg))
	if err != nil {
		return dependencyError(bench.StageForeignImport, err)
	}
	defer st.Close()
	before, err := bench.InspectTree(canonicalRoot(cfg))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(adapterRoot(cfg), 0o755); err != nil {
		return err
	}
	report, err := archivev2.Export(context.Background(), st, archiveRoot(cfg), archivev2.ExportOptions{
		Pack: cfg.adapter == "archive-v2-pack", SkipVerification: true, Overwrite: true,
	})
	if err != nil {
		return err
	}
	after, err := bench.InspectTree(canonicalRoot(cfg))
	if err != nil {
		return err
	}
	if !bench.SameInventory(before, after) {
		return fmt.Errorf("canonical source changed during export")
	}
	result.Source = before
	result.Output, err = bench.InspectTree(archiveRoot(cfg))
	if err != nil {
		return err
	}
	result.Metrics.Items = int64(report.Counts.Documents)
	result.Metrics.Resources = int64(report.ReachableResources)
	result.Metrics.InputBytes, result.Metrics.OutputBytes = before.ApparentBytes, result.Output.ApparentBytes
	result.ArtifactSHA256, err = hashTree(archiveRoot(cfg))
	return err
}

func snapshotVerify(cfg configuration, result *bench.Result) error {
	before, err := bench.InspectTree(archiveRoot(cfg))
	if err != nil {
		return dependencyError(bench.StageSnapshotCreate, err)
	}
	report, err := archivev2.VerifyDirectory(archiveRoot(cfg), archivev2.DefaultLimits())
	if err != nil {
		return err
	}
	after, err := bench.InspectTree(archiveRoot(cfg))
	if err != nil {
		return err
	}
	if !bench.SameInventory(before, after) {
		return fmt.Errorf("archive changed during verification")
	}
	result.Source, result.Output = before, after
	result.Metrics.Items, result.Metrics.InputBytes, result.Metrics.OutputBytes = int64(report.Records), before.ApparentBytes, after.ApparentBytes
	result.Metrics.Resources = int64(report.Counts.Resources)
	result.ArtifactSHA256, err = hashTree(archiveRoot(cfg))
	result.Assertions.ExactHashVerified = err == nil
	if err == nil {
		result.Assertions.CanonicalFingerprint, err = semanticFingerprint(archiveRoot(cfg))
	}
	return err
}

func transportPrepare(cfg configuration, result *bench.Result) error {
	before, err := bench.InspectTree(archiveRoot(cfg))
	if err != nil {
		return dependencyError(bench.StageSnapshotVerify, err)
	}
	temporary := zipPath(cfg) + ".partial"
	_ = os.Remove(temporary)
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, packErr := syncbackup.Pack(archiveRoot(cfg), file)
	if closeErr := file.Close(); packErr == nil {
		packErr = closeErr
	}
	if packErr != nil {
		return packErr
	}
	if err := os.Rename(temporary, zipPath(cfg)); err != nil {
		return err
	}
	after, err := bench.InspectTree(archiveRoot(cfg))
	if err != nil || !bench.SameInventory(before, after) {
		return fmt.Errorf("archive changed during transport preparation: %v", err)
	}
	result.Source, result.Output = before, singleFileInventory(zipPath(cfg))
	result.Metrics.Items, result.Metrics.InputBytes, result.Metrics.OutputBytes = before.Files, before.ApparentBytes, result.Output.ApparentBytes
	result.Metrics.ContainerEntries = before.Files
	result.ArtifactSHA256, err = hashFile(zipPath(cfg))
	result.Assertions.ExactHashVerified = err == nil
	return err
}

func transportSeal(cfg configuration, result *bench.Result) error {
	zipInventory := singleFileInventory(zipPath(cfg))
	if zipInventory.Files != 1 {
		return dependencyError(bench.StageTransportPrepare, os.ErrNotExist)
	}
	key, err := sealingKey(cfg)
	if err != nil {
		return err
	}
	source, err := os.Open(zipPath(cfg))
	if err != nil {
		return err
	}
	defer source.Close()
	temporary := sealedPath(cfg) + ".partial"
	_ = os.Remove(temporary)
	out, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, sealedHash, sealErr := syncbackup.Seal(source, key, out)
	if closeErr := out.Close(); sealErr == nil {
		sealErr = closeErr
	}
	if sealErr != nil {
		return sealErr
	}
	if err := os.Rename(temporary, sealedPath(cfg)); err != nil {
		return err
	}
	result.Source, result.Output = zipInventory, singleFileInventory(sealedPath(cfg))
	result.Metrics.Items, result.Metrics.InputBytes, result.Metrics.OutputBytes = 1, zipInventory.ApparentBytes, result.Output.ApparentBytes
	result.SealedSHA256 = sealedHash
	actual, err := hashFile(sealedPath(cfg))
	result.Assertions.ExactHashVerified = err == nil && actual == sealedHash
	return err
}

func snapshotOpen(cfg configuration, result *bench.Result) error {
	key, err := sealingKey(cfg)
	if err != nil {
		return dependencyError(bench.StageTransportSeal, err)
	}
	source, err := os.Open(sealedPath(cfg))
	if err != nil {
		return dependencyError(bench.StageTransportSeal, err)
	}
	defer source.Close()
	if err := os.RemoveAll(openRoot(cfg)); err != nil {
		return err
	}
	if err := os.MkdirAll(openRoot(cfg), 0o755); err != nil {
		return err
	}
	openedZip := filepath.Join(openRoot(cfg), "snapshot.zip")
	out, err := os.OpenFile(openedZip, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	openErr := syncbackup.Open(source, key, out)
	if closeErr := out.Close(); openErr == nil {
		openErr = closeErr
	}
	if openErr != nil {
		return openErr
	}
	extracted := filepath.Join(openRoot(cfg), "archive")
	if err := syncbackup.Unpack(openedZip, extracted); err != nil {
		return err
	}
	if _, err := archivev2.VerifyDirectory(extracted, archivev2.DefaultLimits()); err != nil {
		return err
	}
	originalHash, err := hashFile(zipPath(cfg))
	if err != nil {
		return err
	}
	openedHash, err := hashFile(openedZip)
	if err != nil {
		return err
	}
	result.Source = singleFileInventory(sealedPath(cfg))
	result.Output, err = bench.InspectTree(openRoot(cfg))
	result.Metrics.Items, result.Metrics.InputBytes, result.Metrics.OutputBytes = result.Output.Files, result.Source.ApparentBytes, result.Output.ApparentBytes
	result.ArtifactSHA256, result.Assertions.ExactHashVerified = openedHash, openedHash == originalHash
	if !result.Assertions.ExactHashVerified {
		return fmt.Errorf("opened wrapper hash differs from prepared wrapper")
	}
	return err
}

func snapshotRestore(cfg configuration, result *bench.Result) error {
	archive := filepath.Join(openRoot(cfg), "archive")
	if _, err := archivev2.VerifyDirectory(archive, archivev2.DefaultLimits()); err != nil {
		return dependencyError(bench.StageSnapshotOpen, err)
	}
	if err := os.MkdirAll(restoredRoot(cfg), 0o755); err != nil {
		return err
	}
	_, existingErr := os.Stat(databasePath(restoredRoot(cfg)))
	existingTarget := existingErr == nil
	if existingErr != nil && !errors.Is(existingErr, os.ErrNotExist) {
		return existingErr
	}
	target, err := openStore(restoredRoot(cfg))
	if err != nil {
		return err
	}
	defer target.Close()
	intent := archivev2.RestoreAdopt
	if existingTarget {
		intent = archivev2.RestoreReplace
	}
	summary, err := archivev2.Restore(context.Background(), target, archive, archivev2.RestoreOptions{Intent: intent})
	if err != nil {
		return err
	}
	// Use P4's content-bearing database aggregate rather than a re-export's
	// record-object hashes. A restored writable copy intentionally rotates its
	// replica identity, so byte-identical record chunks are not the contract.
	want, err := databaseContentFingerprint(databasePath(canonicalRoot(cfg)))
	if err != nil {
		return err
	}
	got, err := databaseContentFingerprint(databasePath(restoredRoot(cfg)))
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("restored canonical content fingerprint differs")
	}
	result.Source, err = bench.InspectTree(archive)
	if err != nil {
		return err
	}
	result.Output, err = bench.InspectTree(restoredRoot(cfg))
	result.Metrics.Items, result.Metrics.InputBytes, result.Metrics.OutputBytes = restoreCount(summary.Applied), result.Source.ApparentBytes, result.Output.ApparentBytes
	result.Metrics.Resources = int64(summary.Applied.Resources)
	result.Assertions.CanonicalFingerprint, result.Assertions.ExactHashVerified = got, true
	return err
}

func incrementalReplay(cfg configuration, result *bench.Result) error {
	source, err := openStore(canonicalRoot(cfg))
	if err != nil {
		return dependencyError(bench.StageForeignImport, err)
	}
	defer source.Close()
	target, err := openStore(restoredRoot(cfg))
	if err != nil {
		return dependencyError(bench.StageSnapshotRestore, err)
	}
	defer target.Close()
	ctx := context.Background()
	if _, err := source.EnrollLocalJournal(ctx, "G14a post-snapshot calibration"); err != nil {
		return err
	}
	if _, err := target.EnrollLocalJournal(ctx, "G14a post-snapshot calibration"); err != nil {
		return err
	}
	sourceHandshake, err := source.LocalSyncHandshake(ctx)
	if err != nil {
		return err
	}
	targetHandshake, err := target.LocalSyncHandshake(ctx)
	if err != nil {
		return err
	}
	keys, err := loadOrCreateSyncKeys(cfg)
	if err != nil {
		return err
	}
	if _, err := source.EnrollPeerSigningKey(ctx, targetHandshake.ReplicaID, keys.Target.Public, "g14a"); err != nil {
		return err
	}
	if _, err := target.EnrollPeerSigningKey(ctx, sourceHandshake.ReplicaID, keys.Source.Public, "g14a"); err != nil {
		return err
	}
	if err := source.ConfigureSyncAdmissionPeer(ctx, targetHandshake); err != nil {
		return err
	}
	if err := target.ConfigureSyncAdmissionPeer(ctx, sourceHandshake); err != nil {
		return err
	}
	const documentID = "doc_g14a_post_snapshot"
	if _, err := source.GetDocument(ctx, documentID); errors.Is(err, store.ErrNotFound) {
		if _, err := source.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: documentID, Title: "Generated post-snapshot calibration", Body: "generated incremental replay\n"}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	group := syncwire.GroupKey{KeyID: "key_g14a", Epoch: 1, Key: keys.Group}
	carrierRoot := filepath.Join(adapterRoot(cfg), "incremental-carrier")
	if err := os.MkdirAll(carrierRoot, 0o755); err != nil {
		return err
	}
	sourceCarrier, err := synccarrier.NewDirectory(carrierRoot, group, sourceHandshake.DatabaseID, sourceHandshake.ReplicaID)
	if err != nil {
		return err
	}
	targetCarrier, err := synccarrier.NewDirectory(carrierRoot, group, targetHandshake.DatabaseID, targetHandshake.ReplicaID)
	if err != nil {
		return err
	}
	sourceRing := &keyRing{group: group, private: keys.Source.Private}
	targetRing := &keyRing{group: group, private: keys.Target.Private}
	sourceRound := synccarrier.NewRound(synccarrier.NewStoreReplica(source), sourceCarrier, sourceRing, sourceRing,
		syncwire.MultiVerifier{sourceRing, source.PeerVerifier()}, store.NewLocalObjectProvider(source), synccarrier.Options{})
	targetRound := synccarrier.NewRound(synccarrier.NewStoreReplica(target), targetCarrier, targetRing, targetRing,
		syncwire.MultiVerifier{targetRing, target.PeerVerifier()}, store.NewLocalObjectProvider(target), synccarrier.Options{})
	for pass := 0; pass < 3; pass++ {
		if _, err := sourceRound.Run(ctx); err != nil {
			return err
		}
		if _, err := targetRound.Run(ctx); err != nil {
			return err
		}
	}
	document, err := target.GetDocument(ctx, documentID)
	if err != nil || document.Body != "generated incremental replay\n" {
		return fmt.Errorf("post-snapshot operation did not converge: %v", err)
	}
	result.Source, err = bench.InspectTree(canonicalRoot(cfg))
	if err != nil {
		return err
	}
	result.Output, err = bench.InspectTree(restoredRoot(cfg))
	result.Metrics.Items, result.Metrics.InputBytes, result.Metrics.OutputBytes = 1, result.Source.ApparentBytes, result.Output.ApparentBytes
	result.Assertions.ExactHashVerified = true
	return err
}

type persistedKey struct {
	Public  []byte `json:"public"`
	Private []byte `json:"private"`
}

type persistedKeys struct {
	Group  [syncwire.GroupKeyBytes]byte `json:"group"`
	Source persistedKey                 `json:"source"`
	Target persistedKey                 `json:"target"`
}

func loadOrCreateSyncKeys(cfg configuration) (persistedKeys, error) {
	path := filepath.Join(adapterRoot(cfg), "private-sync-keys.json")
	var keys persistedKeys
	if raw, err := os.ReadFile(path); err == nil {
		err = json.Unmarshal(raw, &keys)
		return keys, err
	} else if !errors.Is(err, os.ErrNotExist) {
		return keys, err
	}
	if _, err := rand.Read(keys.Group[:]); err != nil {
		return keys, err
	}
	for _, target := range []*persistedKey{&keys.Source, &keys.Target} {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return keys, err
		}
		target.Public, target.Private = public, private
	}
	return keys, writePrivateJSON(path, keys)
}

type keyRing struct {
	group   syncwire.GroupKey
	private ed25519.PrivateKey
}

func (k *keyRing) Current() (syncwire.GroupKey, error)              { return k.group, nil }
func (k *keyRing) Lookup(string, uint32) (syncwire.GroupKey, error) { return k.group, nil }
func (k *keyRing) Sign(message []byte) []byte                       { return ed25519.Sign(k.private, message) }
func (k *keyRing) SignerKeyID() string {
	return syncwire.SignerKeyID(k.private.Public().(ed25519.PublicKey))
}
func (k *keyRing) PublicKey(id string) (ed25519.PublicKey, bool) {
	if id != k.SignerKeyID() {
		return nil, false
	}
	return k.private.Public().(ed25519.PublicKey), true
}

func ensureGeneratedSource(cfg configuration) error {
	root := sourceRoot(cfg)
	if err := os.MkdirAll(filepath.Join(root, "resources"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "relations"), 0o755); err != nil {
		return err
	}
	progressPath := filepath.Join(filepath.Dir(root), "source-progress")
	start := 0
	if raw, err := os.ReadFile(progressPath); err == nil {
		start, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
	}
	const folderCount, tagCount, resourceCount = 10, 20, 20
	if start == 0 {
		for index := 0; index < folderCount; index++ {
			parent := ""
			if index > 0 {
				parent = fmt.Sprintf("parent_id: folder-%02d\n", index-1)
			}
			if err := writeGenerated(filepath.Join(root, fmt.Sprintf("folder-%02d.md", index)), fmt.Sprintf("id: folder-%02d\n%stitle: Folder %02d\ntype_: 2\n", index, parent, index)); err != nil {
				return err
			}
		}
		for index := 0; index < tagCount; index++ {
			if err := writeGenerated(filepath.Join(root, fmt.Sprintf("tag-%02d.md", index)), fmt.Sprintf("id: tag-%02d\ntitle: tag-%02d\ntype_: 5\n", index, index)); err != nil {
				return err
			}
		}
		for index := 0; index < resourceCount; index++ {
			if err := writeGenerated(filepath.Join(root, fmt.Sprintf("resource-%02d.md", index)), fmt.Sprintf("id: resource-%02d\ntitle: resource-%02d.bin\nfilename: resource-%02d.bin\ntype_: 4\n", index, index, index)); err != nil {
				return err
			}
			if err := writeGenerated(filepath.Join(root, "resources", fmt.Sprintf("resource-%02d", index)), fmt.Sprintf("generated resource %d", index)); err != nil {
				return err
			}
		}
	}
	for index := start; index < cfg.tier; index++ {
		id := fmt.Sprintf("profile-%06d", index)
		if err := writeGenerated(filepath.Join(root, id+".md"), fmt.Sprintf("Generated profile body %d\n\n![resource](:/resource-%02d)\n\nid: %s\nparent_id: folder-%02d\ntitle: Generated profile %d\ntype_: 1\n", index, index%resourceCount, id, index%folderCount, index)); err != nil {
			return err
		}
		if err := writeGenerated(filepath.Join(root, "relations", id+".md"), fmt.Sprintf("id: relation-%06d\nnote_id: %s\ntag_id: tag-%02d\ntype_: 6\n", index, id, index%tagCount)); err != nil {
			return err
		}
		if (index+1)%500 == 0 {
			if err := writePrivateFile(progressPath, []byte(strconv.Itoa(index+1)+"\n")); err != nil {
				return err
			}
		}
	}
	return writePrivateFile(progressPath, []byte(strconv.Itoa(cfg.tier)+"\n"))
}

func writeGenerated(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func openStore(root string) (*store.SQLiteStore, error) {
	st, err := store.OpenSQLiteWithAssetStore(databasePath(root), filepath.Join(root, "assets"))
	if err != nil {
		return nil, err
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		_ = st.Close()
		return nil, err
	}
	return st, nil
}

func semanticFingerprint(root string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return "", err
	}
	var manifest archivev2.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return "", err
	}
	identities := []string{}
	for _, descriptor := range manifest.Index {
		if descriptor.Location.Path == "" {
			return "", fmt.Errorf("index chunk is not directly addressable")
		}
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(descriptor.Location.Path)))
		if err != nil {
			return "", err
		}
		scanner := bufio.NewScanner(file)
		buffer := make([]byte, 64*1024)
		scanner.Buffer(buffer, 8<<20)
		for scanner.Scan() {
			var entry archivev2.IndexEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				_ = file.Close()
				return "", err
			}
			if entry.Kind != "pack" {
				identities = append(identities, entry.Kind+":"+entry.SHA256)
			}
		}
		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return "", scanErr
		}
	}
	sort.Strings(identities)
	digest := sha256.New()
	for _, identity := range identities {
		_, _ = io.WriteString(digest, identity+"\n")
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func databaseContentFingerprint(database string) (string, error) {
	// These are the 12 complete canonical counts plus exact ordered content
	// identities used by P4. Rows stream into SHA-256; they are never retained
	// in a benchmark result or printed.
	const query = `
SELECT 'documents=' || COUNT(*) FROM documents;
SELECT 'revisions=' || COUNT(*) FROM document_revisions;
SELECT 'resources=' || COUNT(*) FROM resources;
SELECT 'blobs=' || COUNT(*) FROM blobs;
SELECT 'document_resources=' || COUNT(*) FROM document_resource_refs;
SELECT 'document_tags=' || COUNT(*) FROM note_tags;
SELECT 'tags=' || COUNT(*) FROM tags;
SELECT 'notebooks=' || COUNT(*) FROM notebooks;
SELECT 'sources=' || COUNT(*) FROM document_sources;
SELECT 'source_bundles=' || COUNT(*) FROM source_bundle_items;
SELECT 'links=' || COUNT(*) FROM document_links;
SELECT 'blob_bytes=' || COALESCE(SUM(size_bytes), 0) FROM blobs;
SELECT 'blob:' || sha256 FROM blobs ORDER BY sha256;
SELECT 'source_bundle:' || sha256 FROM source_bundle_items ORDER BY sha256;
`
	command := exec.Command("sqlite3", "-batch", database, query)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr strings.Builder
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return "", err
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, stdout); err != nil {
		_ = command.Wait()
		return "", err
	}
	if err := command.Wait(); err != nil {
		return "", fmt.Errorf("sqlite aggregate failed: %s", strings.TrimSpace(stderr.String()))
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func hashTree(root string) (string, error) {
	entries := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digest, err := hashFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, filepath.ToSlash(relative)+":"+digest)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(entries)
	digest := sha256.New()
	for _, entry := range entries {
		_, _ = io.WriteString(digest, entry+"\n")
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func sealingKey(cfg configuration) ([]byte, error) {
	path := filepath.Join(adapterRoot(cfg), "private-sealing-key")
	if key, err := os.ReadFile(path); err == nil {
		if len(key) != syncbackup.KeyBytes {
			return nil, fmt.Errorf("stored sealing key has wrong length")
		}
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, syncbackup.KeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, writePrivateFile(path, key)
}

func writePrivateJSON(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writePrivateFile(path, raw)
}

func writePrivateFile(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".partial"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func singleFileInventory(path string) bench.Inventory {
	info, err := os.Stat(path)
	if err != nil {
		return bench.Inventory{}
	}
	result := bench.Inventory{Files: 1, ApparentBytes: info.Size(), MaximumDirectorySize: 1, MetadataSHA256: "single-file"}
	result.DirectoryHistogram.OneToTen = 0
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		result.AllocatedBytes = stat.Blocks * 512
	}
	return result
}

type metricStart struct {
	time            time.Time
	usage           syscall.Rusage
	ioRead, ioWrite int64
}

func startMetrics() metricStart {
	var usage syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	read, write := processIO()
	return metricStart{time: time.Now(), usage: usage, ioRead: read, ioWrite: write}
}

func finishMetrics(start metricStart, metrics *bench.Metrics) {
	var usage syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	read, write := processIO()
	metrics.WallSeconds = time.Since(start.time).Seconds()
	metrics.UserCPUSeconds = timevalSeconds(usage.Utime) - timevalSeconds(start.usage.Utime)
	metrics.SystemCPUSeconds = timevalSeconds(usage.Stime) - timevalSeconds(start.usage.Stime)
	metrics.PeakRSSBytes = usage.Maxrss * 1024
	metrics.ReadBytes, metrics.WriteBytes = max64(0, read-start.ioRead), max64(0, write-start.ioWrite)
}

func processIO() (int64, int64) {
	raw, err := os.ReadFile("/proc/self/io")
	if err != nil {
		return 0, 0
	}
	var read, write int64
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		value, _ := strconv.ParseInt(fields[1], 10, 64)
		if fields[0] == "read_bytes:" {
			read = value
		}
		if fields[0] == "write_bytes:" {
			write = value
		}
	}
	return read, write
}

func timevalSeconds(value syscall.Timeval) float64 {
	return float64(value.Sec) + float64(value.Usec)/1e6
}
func max64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func aggregateCanonicalFingerprint(metrics store.SQLiteImportMetrics) string {
	raw, _ := json.Marshal(metrics)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func restoreCount(counts store.RestoreCounts) int64 {
	return int64(counts.Collections + counts.Notebooks + counts.SearchNotebooks + counts.Tags + counts.Documents +
		counts.Revisions + counts.DocumentTags + counts.Resources + counts.DocumentResources + counts.Links +
		counts.Provenance + counts.SourceBundles)
}

func validateResults(root string) error {
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" || !strings.Contains(filepath.ToSlash(path), "/results/") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var result bench.Result
		if err := json.Unmarshal(raw, &result); err != nil {
			return err
		}
		if err := bench.ValidateResult(result); err != nil {
			return fmt.Errorf("result %d: %w", count+1, err)
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no result rows found")
	}
	fmt.Printf("validated %d aggregate phase results\n", count)
	return nil
}

func sourceRoot(cfg configuration) string {
	return filepath.Join(cfg.workspace, "data", fmt.Sprintf("generated-%d", cfg.tier), "joplin-raw")
}
func canonicalRoot(cfg configuration) string {
	return filepath.Join(cfg.workspace, "data", fmt.Sprintf("generated-%d", cfg.tier), "canonical")
}
func adapterRoot(cfg configuration) string {
	return filepath.Join(cfg.workspace, "artifacts", fmt.Sprintf("generated-%d", cfg.tier), cfg.adapter)
}
func archiveRoot(cfg configuration) string  { return filepath.Join(adapterRoot(cfg), "archive") }
func openRoot(cfg configuration) string     { return filepath.Join(adapterRoot(cfg), "opened") }
func restoredRoot(cfg configuration) string { return filepath.Join(adapterRoot(cfg), "restored") }
func zipPath(cfg configuration) string      { return filepath.Join(adapterRoot(cfg), "snapshot.zip") }
func sealedPath(cfg configuration) string   { return filepath.Join(adapterRoot(cfg), "snapshot.nbk") }
func databasePath(root string) string       { return filepath.Join(root, "notes.sqlite") }

func dependencyError(stage bench.Stage, err error) error {
	return fmt.Errorf("phase dependency %s is incomplete: %w", stage, err)
}

func repositoryRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	must(encoder.Encode(value))
}

func must(err error) {
	if err != nil {
		fail(err.Error())
	}
}
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
