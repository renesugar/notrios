package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/synckeys"
	"github.com/renesugar/notrios/internal/syncstate"
)

// Pairing, as a user performs it.
//
// One ceremony, two carriers. Online, `sync invite` prints a code and
// `sync join` spends it in a single exchange. Offline — a replica that will
// only ever meet its peer through a folder or a USB drive — `sync invite
// --offline` writes a file whose group key is *wrapped* under the same code,
// and the two halves travel separately: the file by whatever means is
// convenient, the code by voice.
//
// That separation is G13's resolved decision made real. v0.7 G11 shipped a
// development bundle with the library's group key in clear text, which is
// exactly what a pairing artifact must not contain; those commands are gone and
// this file replaces them.

// offlineInvite is the file half of an offline pairing. Alone it is useless:
// the group key inside is sealed under the code, and the proof binds it.
type offlineInvite struct {
	Version      int    `json:"version"`
	InvitationID string `json:"invitation_id"`
	DatabaseID   string `json:"database_id"`
	ReplicaID    string `json:"replica_id"`
	PublicKey    string `json:"public_key"`
	KeyID        string `json:"key_id"`
	Epoch        uint32 `json:"epoch"`
	WrappedGroup string `json:"wrapped_group_key"`
	Proof        string `json:"proof"`
	ExpiresAt    string `json:"expires_at"`
	SchemaVerson int    `json:"schema_version"`
}

// offlineAcceptance is what the joining replica hands back so the inviter can
// enrol it. It carries no secret at all — a public key and a proof of having
// held the code.
type offlineAcceptance struct {
	Version      int    `json:"version"`
	InvitationID string `json:"invitation_id"`
	DatabaseID   string `json:"database_id"`
	ReplicaID    string `json:"replica_id"`
	PublicKey    string `json:"public_key"`
	Proof        string `json:"proof"`
}

func runSyncInvite(args []string) {
	flags := newSyncFlags("invite")
	ttl := flags.set.Duration("ttl", 15*time.Minute, "how long the code stays usable")
	label := flags.set.String("label", "", "a note to yourself about who this is for")
	offline := flags.set.Bool("offline", false, "also write a file half, for a peer that has no network path to this one")
	out := flags.set.String("out", "", "where to write the offline invite file")
	flags.parse(args)
	if *offline && strings.TrimSpace(*out) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync invite --offline --out <file>")
		os.Exit(2)
	}
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()
	requireEnrolled(status)
	keys := mustOpenKeys(flags.keyPath(databaseID))

	secret, err := syncauth.NewPairingSecret()
	if err != nil {
		exit(err)
	}
	invitation, err := st.CreatePairingInvitation(context.Background(), secret, *ttl, *label)
	if err != nil {
		exit(err)
	}
	report := map[string]any{
		"invitation_id": invitation.ID,
		"code":          syncauth.FormatCode(secret),
		"expires_at":    invitation.ExpiresAt,
		"single_use":    true,
	}
	if *offline {
		group, err := keys.Current()
		if err != nil {
			exit(err)
		}
		wrapped, err := syncauth.WrapGroupKey(secret, group.Key[:])
		if err != nil {
			exit(err)
		}
		handshake, err := st.LocalSyncHandshake(context.Background())
		if err != nil {
			exit(err)
		}
		invite := offlineInvite{
			Version: synckeys.FileVersion, InvitationID: invitation.ID, DatabaseID: databaseID,
			ReplicaID: handshake.ReplicaID, PublicKey: syncauth.EncodePublicKey(keys.PublicSigningKey()),
			KeyID: group.KeyID, Epoch: group.Epoch, WrappedGroup: wrapped,
			ExpiresAt: invitation.ExpiresAt, SchemaVerson: store.CurrentSchemaVersion,
		}
		if invite.Proof, err = syncauth.ProveResponse(secret, syncauth.PairingResponse{
			DatabaseID: invite.DatabaseID, ReplicaID: invite.ReplicaID, PublicKey: invite.PublicKey,
			KeyID: invite.KeyID, WrappedGroup: invite.WrappedGroup,
		}); err != nil {
			exit(err)
		}
		writeJSONFile(*out, invite, 0o600)
		report["invite_file"] = *out
	}
	fmt.Fprintln(os.Stderr, "Read the code to the other device. Do not send it the same way you send the file:")
	fmt.Fprintln(os.Stderr, "  "+syncauth.FormatCode(secret))
	fmt.Fprintln(os.Stderr, "It works once, and only until it expires.")
	printJSON(report)
}

func runSyncJoin(args []string) {
	flags := newSyncFlags("join")
	peerURL := flags.set.String("url", "", "the inviting replica's base URL")
	code := flags.set.String("code", "", "the pairing code that replica displayed")
	flags.parse(args)
	if strings.TrimSpace(*peerURL) == "" || strings.TrimSpace(*code) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync join --url <base-url> --code <code>")
		os.Exit(2)
	}
	secret, err := syncauth.ParseCode(*code)
	if err != nil {
		exit(err)
	}
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()
	requireEnrolled(status)
	keys := mustOpenKeys(flags.keyPath(databaseID))

	request := syncauth.PairingRequest{
		InvitationID: store.InvitationID(secret), DatabaseID: "",
		ReplicaID: status.ReplicaID, PublicKey: syncauth.EncodePublicKey(keys.PublicSigningKey()),
		SchemaVerion: store.CurrentSchemaVersion,
	}
	if request.Proof, err = syncauth.ProveRequest(secret, request); err != nil {
		exit(err)
	}
	code2, body, err := syncauth.Pair(context.Background(), &http.Client{Timeout: 30 * time.Second},
		*peerURL, *code, request)
	if err != nil {
		exit(err)
	}
	if code2 != http.StatusOK {
		fmt.Fprintf(os.Stderr, "the peer refused the pairing (HTTP %d): %s\n", code2, strings.TrimSpace(string(body)))
		os.Exit(1)
	}
	var response syncauth.PairingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		exit(err)
	}
	// The response proof is the joiner's only assurance that it reached the
	// replica whose code it was given, rather than whatever answered the
	// address it dialled.
	if err := syncauth.VerifyResponseProof(secret, response); err != nil {
		exit(fmt.Errorf("the peer's answer does not prove it holds this code: %w", err))
	}
	adopt(st, keys, databaseID, response.DatabaseID, response.ReplicaID, response.PublicKey,
		response.KeyID, response.Epoch, response.WrappedGroup, secret, response.SchemaVersion)
	printJSON(map[string]any{
		"paired_with":     response.ReplicaID,
		"database_id":     response.DatabaseID,
		"signer_key_id":   response.InviterKeyID,
		"public_base_url": response.PublicBaseURL,
	})
}

func runSyncAccept(args []string) {
	flags := newSyncFlags("accept")
	invitePath := flags.set.String("invite", "", "the invite file from the other replica")
	code := flags.set.String("code", "", "the pairing code that replica displayed")
	out := flags.set.String("out", "", "where to write the acceptance file to hand back")
	flags.parse(args)
	if strings.TrimSpace(*invitePath) == "" || strings.TrimSpace(*code) == "" || strings.TrimSpace(*out) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync accept --invite <file> --code <code> --out <file>")
		os.Exit(2)
	}
	secret, err := syncauth.ParseCode(*code)
	if err != nil {
		exit(err)
	}
	var invite offlineInvite
	readJSONFile(*invitePath, &invite)
	if err := syncauth.VerifyResponseProof(secret, syncauth.PairingResponse{
		DatabaseID: invite.DatabaseID, ReplicaID: invite.ReplicaID, PublicKey: invite.PublicKey,
		KeyID: invite.KeyID, WrappedGroup: invite.WrappedGroup, ResponseProof: invite.Proof,
	}); err != nil {
		exit(fmt.Errorf("this invite does not match that code: %w", err))
	}
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()
	requireEnrolled(status)
	keys := mustOpenKeys(flags.keyPath(databaseID))

	adopt(st, keys, databaseID, invite.DatabaseID, invite.ReplicaID, invite.PublicKey,
		invite.KeyID, invite.Epoch, invite.WrappedGroup, secret, invite.SchemaVerson)

	acceptance := offlineAcceptance{
		Version: synckeys.FileVersion, InvitationID: invite.InvitationID, DatabaseID: invite.DatabaseID,
		ReplicaID: status.ReplicaID, PublicKey: syncauth.EncodePublicKey(keys.PublicSigningKey()),
	}
	if acceptance.Proof, err = syncauth.ProveRequest(secret, syncauth.PairingRequest{
		InvitationID: acceptance.InvitationID, DatabaseID: acceptance.DatabaseID,
		ReplicaID: acceptance.ReplicaID, PublicKey: acceptance.PublicKey,
	}); err != nil {
		exit(err)
	}
	writeJSONFile(*out, acceptance, 0o600)
	printJSON(map[string]any{
		"paired_with":  invite.ReplicaID,
		"database_id":  invite.DatabaseID,
		"acceptance":   *out,
		"hand_back_to": invite.ReplicaID,
	})
}

func runSyncEnroll(args []string) {
	flags := newSyncFlags("enroll")
	acceptancePath := flags.set.String("acceptance", "", "the acceptance file the other replica produced")
	code := flags.set.String("code", "", "the pairing code you displayed")
	flags.parse(args)
	if strings.TrimSpace(*acceptancePath) == "" || strings.TrimSpace(*code) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync enroll --acceptance <file> --code <code>")
		os.Exit(2)
	}
	secret, err := syncauth.ParseCode(*code)
	if err != nil {
		exit(err)
	}
	var acceptance offlineAcceptance
	readJSONFile(*acceptancePath, &acceptance)
	if err := syncauth.VerifyRequestProof(secret, syncauth.PairingRequest{
		InvitationID: acceptance.InvitationID, DatabaseID: acceptance.DatabaseID,
		ReplicaID: acceptance.ReplicaID, PublicKey: acceptance.PublicKey, Proof: acceptance.Proof,
	}); err != nil {
		exit(fmt.Errorf("this acceptance does not match that code: %w", err))
	}
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()
	requireEnrolled(status)
	if acceptance.DatabaseID != databaseID {
		exit(fmt.Errorf("that acceptance is for database %s, not %s", acceptance.DatabaseID, databaseID))
	}
	ctx := context.Background()
	// Spending the invitation happens here, in one transaction, so an offline
	// code is single-use exactly as an online one is.
	if _, err := st.ConsumePairingInvitation(ctx, secret, acceptance.ReplicaID, time.Now()); err != nil {
		exit(fmt.Errorf("that code is no longer usable: %w", err))
	}
	public, err := syncauth.DecodePublicKey(acceptance.PublicKey)
	if err != nil {
		exit(err)
	}
	keyID, err := st.EnrollPeerSigningKey(ctx, acceptance.ReplicaID, public, "paired offline")
	if err != nil {
		exit(err)
	}
	if err := st.ConfigureSyncAdmissionPeer(ctx, syncstate.NewHandshake(databaseID, acceptance.ReplicaID,
		store.CurrentSchemaVersion, syncstate.Vector{acceptance.ReplicaID: 0})); err != nil {
		exit(err)
	}
	printJSON(map[string]any{
		"enrolled_replica_id": acceptance.ReplicaID,
		"signer_key_id":       keyID,
		"database_id":         databaseID,
	})
}

// adopt performs the joining side of any pairing: take the group key, trust the
// inviter's signing key, and configure it for admission.
func adopt(st *store.SQLiteStore, keys *synckeys.KeyFile, localDatabaseID, remoteDatabaseID,
	remoteReplicaID, remotePublicKey, keyID string, epoch uint32, wrapped string,
	secret []byte, schemaVersion int) {
	if remoteDatabaseID != localDatabaseID {
		exit(fmt.Errorf("that replica belongs to database %s, not %s — a peer of another library is not a peer",
			remoteDatabaseID, localDatabaseID))
	}
	group, err := syncauth.UnwrapGroupKey(secret, wrapped)
	if err != nil {
		exit(err)
	}
	if err := keys.AdoptGroupKey(keyID, epoch, group); err != nil {
		exit(err)
	}
	public, err := syncauth.DecodePublicKey(remotePublicKey)
	if err != nil {
		exit(err)
	}
	ctx := context.Background()
	if _, err := st.EnrollPeerSigningKey(ctx, remoteReplicaID, public, "paired"); err != nil {
		exit(err)
	}
	if schemaVersion == 0 {
		schemaVersion = store.CurrentSchemaVersion
	}
	if err := st.ConfigureSyncAdmissionPeer(ctx, syncstate.NewHandshake(localDatabaseID, remoteReplicaID,
		schemaVersion, syncstate.Vector{remoteReplicaID: 0})); err != nil {
		exit(err)
	}
}

func runSyncPeers(args []string) {
	flags := newSyncFlags("peers")
	flags.parse(args)
	st, _, _ := flags.openSyncStore()
	defer st.Close()
	ctx := context.Background()
	keys, err := st.ListPeerSigningKeys(ctx)
	if err != nil {
		exit(err)
	}
	peers, err := st.ListSyncPeers(ctx)
	if err != nil {
		exit(err)
	}
	admission := map[string]bool{}
	for _, peer := range peers {
		admission[peer.Handshake.ReplicaID] = true
	}
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, map[string]any{
			"replica_id":    key.ReplicaID,
			"signer_key_id": key.SignerKeyID,
			"status":        key.Status,
			"enrolled_at":   key.EnrolledAt,
			"revoked_at":    key.RevokedAt,
			"admits":        admission[key.ReplicaID],
		})
	}
	invitations, err := st.ListPairingInvitations(ctx, 20)
	if err != nil {
		exit(err)
	}
	printJSON(map[string]any{"peers": rows, "invitations": invitations})
}

func runSyncRevoke(args []string) {
	flags := newSyncFlags("revoke")
	keyID := flags.set.String("key", "", "the signer key id to stop trusting")
	reason := flags.set.String("reason", "", "why, for the audit record")
	advance := flags.set.Bool("advance-epoch", false, "also mint a new group key epoch, so the revoked device cannot read what is published next")
	flags.parse(args)
	if strings.TrimSpace(*keyID) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync revoke --key <signer-key-id> [--reason ...] [--advance-epoch]")
		os.Exit(2)
	}
	st, _, databaseID := flags.openSyncStore()
	defer st.Close()
	if err := st.RevokePeerSigningKey(context.Background(), *keyID, *reason); err != nil {
		exit(err)
	}
	report := map[string]any{"revoked_key": *keyID}
	if *advance {
		// Revoking a credential stops that device signing. Advancing the epoch
		// stops it *reading* what is published afterwards, which is the other
		// half of the answer and deliberately a separate decision: it costs
		// every remaining peer a re-key, and the old epoch stays readable so
		// the library keeps its own history.
		keys := mustOpenKeys(flags.keyPath(databaseID))
		epoch, err := keys.AdvanceEpoch()
		if err != nil {
			exit(err)
		}
		report["new_epoch"] = epoch
		report["remaining_peers_must_repair"] = true
		fmt.Fprintln(os.Stderr, "The group key epoch advanced. Every remaining peer must pair again to receive it.")
	}
	printJSON(report)
}

func requireEnrolled(status store.SyncJournalStatus) {
	if !status.Enabled {
		fmt.Fprintln(os.Stderr, "this library is not enrolled for sync: run `notriosctl sync init` first")
		os.Exit(2)
	}
}

func writeJSONFile(path string, value any, mode os.FileMode) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		exit(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), mode); err != nil {
		exit(err)
	}
}

func readJSONFile(path string, value any) {
	contents, err := os.ReadFile(path)
	if err != nil {
		exit(err)
	}
	if err := json.Unmarshal(contents, value); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
