package archivev2

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestACollectionRecordIsWrittenWithoutCapabilities is v0.8 H16-G. The field
// held the same five words for every collection in every archive ever written,
// so it described the code rather than the library.
func TestACollectionRecordIsWrittenWithoutCapabilities(t *testing.T) {
	encoded, err := json.Marshal(CollectionRecord{
		ID: "col_a", Name: "Joplin export, January 2026",
		SettingsJSON: json.RawMessage(`{}`), CreatedAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("encoding a collection record: %v", err)
	}
	if strings.Contains(string(encoded), "capabilities") {
		t.Errorf("a new collection record still carries capabilities: %s", encoded)
	}
}

// TestAnOlderArchiveWithCapabilitiesStillReads is the direction that would have
// lost data. decodeStrict refuses unknown fields, so deleting the field outright
// would have made this reader reject every archive Notrios had already written.
func TestAnOlderArchiveWithCapabilitiesStillReads(t *testing.T) {
	older := []byte(`{"id":"col_a","name":"Joplin","capabilities":["documents","graph","links","resources","search"],` +
		`"settings_json":{},"created_at":"2026-01-01T00:00:00Z"}`)

	var record CollectionRecord
	if err := decodeStrict(older, &record); err != nil {
		t.Fatalf("an archive written before H16 was refused: %v", err)
	}
	if record.ID != "col_a" || record.Name != "Joplin" {
		t.Errorf("the record decoded wrongly: %+v", record)
	}
	if len(record.Capabilities) != 5 {
		t.Errorf("the older field was dropped rather than read: %+v", record.Capabilities)
	}
}

// TestAnArchiveWithoutCapabilitiesReads is the new shape read back.
func TestAnArchiveWithoutCapabilitiesReads(t *testing.T) {
	current := []byte(`{"id":"col_a","name":"Joplin","settings_json":{},"created_at":"2026-01-01T00:00:00Z"}`)

	var record CollectionRecord
	if err := decodeStrict(current, &record); err != nil {
		t.Fatalf("an archive written after H16 was refused: %v", err)
	}
	if len(record.Capabilities) != 0 {
		t.Errorf("capabilities appeared from nowhere: %+v", record.Capabilities)
	}
}
