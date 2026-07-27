package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"fmt"
)

// explainQueryPlan is an internal test/benchmark diagnostic. It is not part of
// Store and is never exposed through REST or MCP.
func (s *SQLiteStore) explainQueryPlan(ctx context.Context, sql string, args ...string) ([]string, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked("EXPLAIN QUERY PLAN " + sql)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, args); err != nil {
		return nil, err
	}
	details := []string{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			details = append(details, columnText(stmt, 3))
		case C.SQLITE_DONE:
			return details, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) sqliteVersion(ctx context.Context) (string, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT sqlite_version()`)
	if err != nil {
		return "", err
	}
	defer C.sqlite3_finalize(stmt)
	if rc := C.sqlite3_step(stmt); rc != C.SQLITE_ROW {
		return "", fmt.Errorf("sqlite version: %w", s.stepErrLocked(rc))
	}
	return columnText(stmt, 0), nil
}
