package store

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const keysetCursorVersion = "k2"

type keysetCursor struct {
	Version   string `json:"v"`
	Mode      string `json:"m"`
	Binding   string `json:"b"`
	Timestamp string `json:"t,omitempty"`
	Score     string `json:"s,omitempty"`
	ID        string `json:"i"`
}

func cursorBinding(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:16])
}

func encodeChronologicalCursor(binding, timestamp, id string) string {
	return encodeKeysetCursor(keysetCursor{
		Version:   keysetCursorVersion,
		Mode:      "chronological",
		Binding:   binding,
		Timestamp: timestamp,
		ID:        id,
	})
}

func decodeChronologicalCursor(encoded, binding string) (timestamp, id string, err error) {
	cursor, err := decodeKeysetCursor(encoded, binding, "chronological")
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(cursor.Timestamp) == "" {
		return "", "", invalidCursor("timestamp boundary is missing")
	}
	return cursor.Timestamp, cursor.ID, nil
}

func encodeRelevanceCursor(binding string, score float64, id string) string {
	return encodeKeysetCursor(keysetCursor{
		Version: keysetCursorVersion,
		Mode:    "relevance",
		Binding: binding,
		Score:   strconv.FormatFloat(score, 'g', -1, 64),
		ID:      id,
	})
}

func decodeRelevanceCursor(encoded, binding string) (score float64, id string, err error) {
	cursor, err := decodeKeysetCursor(encoded, binding, "relevance")
	if err != nil {
		return 0, "", err
	}
	score, err = strconv.ParseFloat(cursor.Score, 64)
	if err != nil {
		return 0, "", invalidCursor("score boundary is invalid")
	}
	return score, cursor.ID, nil
}

func encodeKeysetCursor(cursor keysetCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeKeysetCursor(encoded, binding, mode string) (keysetCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return keysetCursor{}, invalidCursor("cursor encoding is invalid")
	}
	var cursor keysetCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return keysetCursor{}, invalidCursor("cursor payload is invalid")
	}
	if cursor.Version != keysetCursorVersion || cursor.Mode != mode ||
		cursor.Binding != binding || strings.TrimSpace(cursor.ID) == "" {
		return keysetCursor{}, invalidCursor("cursor does not match this query and sort")
	}
	return cursor, nil
}

func invalidCursor(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidCursor, message)
}
