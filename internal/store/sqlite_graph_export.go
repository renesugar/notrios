package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"encoding/csv"
	"io"
	"strconv"
)

// Graph export for tools built to analyse graphs.
//
// Notrios should not become Gephi. Centrality, modularity, and community
// detection are whole fields with software designed for them — Gephi handles
// hundreds of thousands of nodes, Cytoscape adds typed edges and semantic
// filtering — and reimplementing any of it inside a notes app would be worse at
// more cost. Emitting the graph is the useful thing to do.
//
// **CSV node and edge lists**, because they stream at any library size without
// holding a document tree in memory, and Gephi, Cytoscape, NetworkX, and igraph
// all import them. Column names follow Gephi's convention (`Id`/`Label`,
// `Source`/`Target`/`Type`) since it is the pickiest of the four; the rest read
// whatever they are given.
//
// The exported graph is the same one the report measures: notes in the Trash
// and in read-only notebooks are left out, along with every link touching them.
// An export that disagreed with the report would be worse than either alone.

// ExportGraphRequest names what to export.
type ExportGraphRequest struct {
	CollectionID string
}

// ExportGraphSummary reports what was written, for a CLI to print.
type ExportGraphSummary struct {
	CollectionID string `json:"collection_id"`
	Nodes        int64  `json:"nodes"`
	Edges        int64  `json:"edges"`
}

// ExportGraphCSV streams the link graph as two CSV tables.
//
// Rows are written as they are read. Nothing accumulates, so a library that
// does not fit in memory still exports, which is the point of choosing CSV over
// a nested format like GraphML or JSON.
func (s *SQLiteStore) ExportGraphCSV(ctx context.Context, req ExportGraphRequest, nodes, edges io.Writer) (ExportGraphSummary, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return ExportGraphSummary{}, err
	}
	collectionID := req.CollectionID
	if collectionID == "" {
		collectionID = "default"
	}
	summary := ExportGraphSummary{CollectionID: collectionID}

	s.mu.Lock()
	defer s.mu.Unlock()

	nodeCount, err := s.exportGraphNodesLocked(ctx, collectionID, nodes)
	if err != nil {
		return ExportGraphSummary{}, err
	}
	summary.Nodes = nodeCount

	edgeCount, err := s.exportGraphEdgesLocked(ctx, collectionID, edges)
	if err != nil {
		return ExportGraphSummary{}, err
	}
	summary.Edges = edgeCount
	return summary, nil
}

func (s *SQLiteStore) exportGraphNodesLocked(ctx context.Context, collectionID string, out io.Writer) (int64, error) {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{"Id", "Label", "Uri", "Notebook", "InDegree", "OutDegree"}); err != nil {
		return 0, err
	}
	excluded := readOnlyNotebookArgs()
	stmt, err := s.prepareLocked(`SELECT d.id, COALESCE(d.title, ''), d.collection_id, COALESCE(d.notebook_id, ''),
			(SELECT COUNT(*) FROM document_links li
				JOIN documents src ON src.id = li.source_document_id
				WHERE li.target_document_id = d.id AND ` + measuredDocumentSQL("src") + `),
			(SELECT COUNT(*) FROM document_links lo
				JOIN documents tgt ON tgt.id = lo.target_document_id
				WHERE lo.source_document_id = d.id AND ` + measuredDocumentSQL("tgt") + `)
		FROM documents d
		WHERE d.collection_id = ? AND ` + measuredDocumentSQL("d") + `
		ORDER BY d.id`)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	args := append([]string{}, excluded...)
	args = append(args, excluded...)
	args = append(args, collectionID)
	args = append(args, excluded...)
	if err := bindAll(stmt, args); err != nil {
		return 0, err
	}

	count := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return 0, s.stepErrLocked(rc)
		}
		id := columnText(stmt, 0)
		label := columnText(stmt, 1)
		if label == "" {
			label = id
		}
		if err := writer.Write([]string{
			id, label,
			DocumentURI(columnText(stmt, 2), id),
			columnText(stmt, 3),
			strconv.FormatInt(columnInt64(stmt, 4), 10),
			strconv.FormatInt(columnInt64(stmt, 5), 10),
		}); err != nil {
			return 0, err
		}
		count++
	}
	writer.Flush()
	return count, writer.Error()
}

func (s *SQLiteStore) exportGraphEdgesLocked(ctx context.Context, collectionID string, out io.Writer) (int64, error) {
	writer := csv.NewWriter(out)
	// "Type" is Gephi's directedness column, not the link kind; the relation
	// type is carried separately so neither has to be guessed at on import.
	if err := writer.Write([]string{"Source", "Target", "Type", "Weight", "Kind"}); err != nil {
		return 0, err
	}
	excluded := readOnlyNotebookArgs()
	stmt, err := s.prepareLocked(`SELECT l.source_document_id, l.target_document_id, COALESCE(l.relation_type, 'link')
		FROM document_links l
		JOIN documents src ON src.id = l.source_document_id
		JOIN documents tgt ON tgt.id = l.target_document_id
		WHERE src.collection_id = ? AND ` + measuredDocumentSQL("src") + ` AND ` + measuredDocumentSQL("tgt") + `
		ORDER BY l.source_document_id, l.target_document_id, l.id`)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	args := []string{collectionID}
	args = append(args, excluded...)
	args = append(args, excluded...)
	if err := bindAll(stmt, args); err != nil {
		return 0, err
	}

	count := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return 0, s.stepErrLocked(rc)
		}
		if err := writer.Write([]string{
			columnText(stmt, 0), columnText(stmt, 1), "Directed", "1", columnText(stmt, 2),
		}); err != nil {
			return 0, err
		}
		count++
	}
	writer.Flush()
	return count, writer.Error()
}
