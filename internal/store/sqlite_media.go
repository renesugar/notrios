package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Remote-media attempt records (v0.3 task H3). Every quarantine fetch —
// successful or refused — leaves one row in media_policy_decisions so policy
// behavior is auditable. Admission to the asset store is a later task (H4).

// MediaAttempt is one recorded remote-media fetch attempt.
type MediaAttempt struct {
	ID             string
	DocumentID     string
	OriginalURL    string
	FinalURL       string
	Decision       string // allow | block | review
	Reason         string
	Status         string // refused | quarantined
	ContentType    string
	SizeBytes      int64
	SHA256         string
	QuarantinePath string
	CreatedAt      string
}

const mediaAttemptColumns = `id, COALESCE(document_id, ''), original_url, COALESCE(final_url, ''), decision, COALESCE(reason, ''), status, COALESCE(content_type, ''), COALESCE(size_bytes, 0), COALESCE(exact_hash, ''), COALESCE(quarantine_path, ''), created_at`

func (s *SQLiteStore) RecordMediaAttempt(ctx context.Context, attempt MediaAttempt) (MediaAttempt, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return MediaAttempt{}, err
	}
	attempt.OriginalURL = strings.TrimSpace(attempt.OriginalURL)
	if attempt.OriginalURL == "" {
		return MediaAttempt{}, fmt.Errorf("%w: original URL is required", ErrInvalidInput)
	}
	if attempt.Status == "" {
		return MediaAttempt{}, fmt.Errorf("%w: attempt status is required", ErrInvalidInput)
	}
	if attempt.ID == "" {
		id, err := NewID("mpd")
		if err != nil {
			return MediaAttempt{}, err
		}
		attempt.ID = id
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.execPreparedLocked(`INSERT INTO media_policy_decisions(id, document_id, original_url, final_url, decision, reason, exact_hash, status, content_type, size_bytes, quarantine_path, updated_at)
		VALUES(?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, NULLIF(?, ''), CURRENT_TIMESTAMP)`,
		attempt.ID,
		attempt.DocumentID,
		attempt.OriginalURL,
		attempt.FinalURL,
		attempt.Decision,
		attempt.Reason,
		attempt.SHA256,
		attempt.Status,
		attempt.ContentType,
		strconv.FormatInt(attempt.SizeBytes, 10),
		attempt.QuarantinePath,
	)
	if err != nil {
		return MediaAttempt{}, err
	}
	return attempt, nil
}

// MediaHashRule blocks or flags content by hash. Exact hashes may block;
// perceptual hashes only ever review (SECURITY_AND_MEDIA_POLICY.md).
type MediaHashRule struct {
	Algo      string
	Hash      string
	Kind      string // exact | perceptual
	Action    string // block | review
	Reason    string
	CreatedAt string
}

func (s *SQLiteStore) AddMediaHashRule(ctx context.Context, rule MediaHashRule) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	rule.Algo = strings.ToLower(strings.TrimSpace(rule.Algo))
	rule.Hash = strings.ToLower(strings.TrimSpace(rule.Hash))
	if rule.Algo == "" || rule.Hash == "" {
		return fmt.Errorf("%w: hash rule requires algo and hash", ErrInvalidInput)
	}
	if rule.Kind == "" {
		rule.Kind = "exact"
	}
	if rule.Kind == "perceptual" && rule.Action == "block" {
		return fmt.Errorf("%w: perceptual hash rules may only review, never block", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execPreparedLocked(`INSERT INTO media_hash_rules(algo, hash, kind, action, reason) VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(algo, hash) DO UPDATE SET kind = excluded.kind, action = excluded.action, reason = excluded.reason`,
		rule.Algo, rule.Hash, rule.Kind, rule.Action, rule.Reason)
}

// FindMediaHashRule returns the rule for one hash, or ErrNotFound.
func (s *SQLiteStore) FindMediaHashRule(ctx context.Context, algo, hash string) (MediaHashRule, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return MediaHashRule{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT algo, hash, kind, action, COALESCE(reason, ''), created_at
		FROM media_hash_rules WHERE algo = ? AND hash = ?`)
	if err != nil {
		return MediaHashRule{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{strings.ToLower(strings.TrimSpace(algo)), strings.ToLower(strings.TrimSpace(hash))}); err != nil {
		return MediaHashRule{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return MediaHashRule{}, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return MediaHashRule{}, s.stepErrLocked(rc)
	}
	return MediaHashRule{
		Algo:      columnText(stmt, 0),
		Hash:      columnText(stmt, 1),
		Kind:      columnText(stmt, 2),
		Action:    columnText(stmt, 3),
		Reason:    columnText(stmt, 4),
		CreatedAt: columnText(stmt, 5),
	}, nil
}

// ListMediaAttempts returns recorded attempts, newest first. An empty
// documentID lists attempts across all documents (including document-less
// check-url evaluations).
func (s *SQLiteStore) ListMediaAttempts(ctx context.Context, documentID string, limit int) ([]MediaAttempt, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	documentID = strings.TrimSpace(documentID)

	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT ` + mediaAttemptColumns + ` FROM media_policy_decisions
		WHERE (? = '' OR document_id = ?)
		ORDER BY created_at DESC, id DESC LIMIT ` + strconv.Itoa(limit))
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID, documentID}); err != nil {
		return nil, err
	}
	attempts := []MediaAttempt{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return attempts, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		size, _ := strconv.ParseInt(columnText(stmt, 8), 10, 64)
		attempts = append(attempts, MediaAttempt{
			ID:             columnText(stmt, 0),
			DocumentID:     columnText(stmt, 1),
			OriginalURL:    columnText(stmt, 2),
			FinalURL:       columnText(stmt, 3),
			Decision:       columnText(stmt, 4),
			Reason:         columnText(stmt, 5),
			Status:         columnText(stmt, 6),
			ContentType:    columnText(stmt, 7),
			SizeBytes:      size,
			SHA256:         columnText(stmt, 9),
			QuarantinePath: columnText(stmt, 10),
			CreatedAt:      columnText(stmt, 11),
		})
	}
}
