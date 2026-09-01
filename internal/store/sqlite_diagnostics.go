package store

/*
#include "csqlite/sqlite3.h"
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
	rows, err := s.queryTextColumn(ctx, `SELECT sqlite_version()`)
	if err != nil {
		return "", fmt.Errorf("sqlite version: %w", err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("sqlite version: no result")
	}
	return rows[0], nil
}

// queryTextColumn runs a fixed diagnostic query and returns its first column
// as text. It takes no arguments by design: this is for constant introspection
// statements such as sqlite_version() and PRAGMA compile_options, not for a
// general query path that could grow into one.
func (s *SQLiteStore) queryTextColumn(ctx context.Context, sql string) ([]string, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(sql)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var rows []string
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_ROW {
			rows = append(rows, columnText(stmt, 0))
			continue
		}
		if rc == C.SQLITE_DONE {
			return rows, nil
		}
		return nil, s.stepErrLocked(rc)
	}
}
