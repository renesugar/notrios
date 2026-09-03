package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/credentials"
	"github.com/renesugar/notrios/internal/synckeys"
)

// `notriosctl sync migrate-credentials` moves existing key material between
// the development file and the operating system's credential store.
//
// It is a command rather than something a configuration change does on its
// own, for the same reason `notriosctl migrate` is: the failure mode is
// silent and permanent. Key material that a peer has already published
// artifacts against cannot be regenerated, so a profile that switched stores
// automatically and could not find its keys afterwards would look exactly like
// a profile that never had any.
//
// The direction is named rather than inferred. "Whatever the file is not" is a
// tempting default and a bad one: a user who ran this twice would move their
// keys back without being told.
func runSyncMigrateCredentials(args []string) {
	flags := newSyncFlags("migrate-credentials")
	to := flags.set.String("to", "", "destination store: native or development-file")
	dryRun := flags.set.Bool("dry-run", false, "report what would happen and stop")
	confirm := flags.set.Bool("confirm", false, "required to actually move key material")
	flags.parse(args)
	destination := strings.ToLower(strings.TrimSpace(*to))
	switch destination {
	case config.CredentialStoreNative, config.CredentialStoreDevelopmentFile:
	case "":
		fmt.Fprintln(os.Stderr, "--to is required: native or development-file")
		os.Exit(2)
	default:
		fmt.Fprintf(os.Stderr, "unknown destination %q: use native or development-file\n", destination)
		os.Exit(2)
	}

	st, _, databaseID := flags.openSyncStore()
	defer st.Close()
	source := flags.keyStore(databaseID)

	plan, err := planCredentialMigration(source, destination, databaseID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("from: %s\nto:   %s\nkeys: %s\n", plan.from, plan.to, source.path)
	if *dryRun {
		fmt.Println("dry run: nothing was moved")
		return
	}
	if !*confirm {
		// The confirmation is a flag rather than a prompt so that the same
		// command works in a script, and so the record of what a user agreed
		// to is in their shell history.
		fmt.Fprintln(os.Stderr, "refusing to move key material without --confirm; re-run with --dry-run to see the plan")
		os.Exit(2)
	}
	if err := plan.execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("moved: key material is now held by %s\n", plan.to)
	if plan.to == "locked-file-development" {
		fmt.Fprintln(os.Stderr, developmentKeyWarning)
	}
}

type credentialMigration struct {
	from, to string
	path     string
	keys     *synckeys.KeyFile
	// provider is the native store involved in whichever direction this goes.
	provider credentials.Provider
	ref      credentials.Reference
	toNative bool
}

// planCredentialMigration opens the current material and checks every refusal
// before anything is written, so a plan that prints is a plan that can run.
func planCredentialMigration(source *syncKeyStore, destination, databaseID string) (*credentialMigration, error) {
	if source.unavailable != nil {
		return nil, fmt.Errorf("cannot read the current key material: %w", source.unavailable)
	}
	keys, err := source.open()
	if err != nil {
		return nil, fmt.Errorf("cannot read the current key material: %w", err)
	}
	toNative := destination == config.CredentialStoreNative
	if toNative == (source.provider != nil) {
		return nil, fmt.Errorf("key material is already held by %s; nothing to do", source.describe())
	}
	provider, err := credentials.Select(credentials.KindNative)
	if err != nil {
		return nil, err
	}
	plan := &credentialMigration{
		from: source.describe(), path: source.path, keys: keys,
		provider: provider, toNative: toNative,
		ref: credentials.SyncReference(databaseID, source.path),
	}
	plan.to = "locked-file-development"
	if toNative {
		plan.to = provider.Name()
		// Refusing an occupied slot rather than overwriting it: a data key
		// already stored for this library belongs to key material somewhere,
		// and replacing it would make that material unopenable forever.
		if _, err := provider.Get(plan.ref); err == nil {
			return nil, fmt.Errorf("%s already holds a data key for this library; "+
				"remove it deliberately before migrating", provider.Name())
		} else if !errors.Is(err, credentials.ErrNotFound) {
			return nil, err
		}
	}
	return plan, nil
}

// execute writes the destination first, verifies it, and only then removes the
// source. Every ordering here is chosen so that an interruption leaves the
// material readable by the store it started in.
func (p *credentialMigration) execute() error {
	staging := p.path + ".migrating"
	_ = os.Remove(staging)
	if p.toNative {
		dataKey, err := synckeys.NewSealKey()
		if err != nil {
			return err
		}
		if err := p.writeStaged(staging, dataKey); err != nil {
			return err
		}
		if err := p.verifyStaged(staging, dataKey); err != nil {
			_ = os.Remove(staging)
			return err
		}
		if err := p.provider.Set(p.ref, dataKey); err != nil {
			_ = os.Remove(staging)
			return fmt.Errorf("storing the data key in %s failed; key material is unchanged: %w",
				p.provider.Name(), err)
		}
		if err := os.Rename(staging, p.path); err != nil {
			// The data key would otherwise be left pointing at material the
			// user does not have, which is indistinguishable from a lost key.
			_ = p.provider.Delete(p.ref)
			_ = os.Remove(staging)
			return err
		}
		return nil
	}

	if err := p.writeStaged(staging, nil); err != nil {
		return err
	}
	if err := p.verifyStaged(staging, nil); err != nil {
		_ = os.Remove(staging)
		return err
	}
	if err := os.Rename(staging, p.path); err != nil {
		_ = os.Remove(staging)
		return err
	}
	// The data key goes last. Until the plaintext file is in place it is the
	// only thing that can open this library's material.
	if err := p.provider.Delete(p.ref); err != nil && !errors.Is(err, credentials.ErrNotFound) {
		return fmt.Errorf("key material was moved to %s but the old data key could not be removed from %s: %w",
			p.path, p.provider.Name(), err)
	}
	return nil
}

// writeStaged re-creates the same material at a staging path, sealed or not.
func (p *credentialMigration) writeStaged(staging string, dataKey []byte) error {
	var (
		written *synckeys.KeyFile
		err     error
	)
	if dataKey != nil {
		written, err = synckeys.CreateSealed(staging, dataKey)
	} else {
		written, err = synckeys.Create(staging)
	}
	if err != nil {
		return err
	}
	return written.ReplaceMaterial(p.keys)
}

// verifyStaged reopens what was written and compares it to the material that
// was read, because "the file exists" is not evidence that the keys survived.
func (p *credentialMigration) verifyStaged(staging string, dataKey []byte) error {
	var (
		reopened *synckeys.KeyFile
		err      error
	)
	if dataKey != nil {
		reopened, err = synckeys.OpenSealed(staging, dataKey)
	} else {
		reopened, err = synckeys.Open(staging)
	}
	if err != nil {
		return fmt.Errorf("the migrated key material did not read back: %w", err)
	}
	if reopened.SignerKeyID() != p.keys.SignerKeyID() {
		return errors.New("the migrated key material has a different signing key")
	}
	if !bytes.Equal(reopened.PrivateSigningKey(), p.keys.PrivateSigningKey()) {
		return errors.New("the migrated signing key does not match")
	}
	want, err := p.keys.Current()
	if err != nil {
		return err
	}
	got, err := reopened.Current()
	if err != nil {
		return err
	}
	if want.Key != got.Key || want.Epoch != got.Epoch || want.KeyID != got.KeyID {
		return errors.New("the migrated group key does not match")
	}
	return nil
}
