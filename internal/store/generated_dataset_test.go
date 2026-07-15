package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestGeneratedDatasetSearchGraphAndResourceSmoke(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	const documentCount = 200
	started := time.Now()
	var first, second Document
	for i := 0; i < documentCount; i++ {
		body := fmt.Sprintf("# Generated %03d\n\nsharedterm generated dataset note needle%03d.\n", i, i)
		if i == 0 {
			body += "\nSee [Generated 001](document://default/documents/perf_doc_001).\n"
		}
		doc, err := st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID:  fmt.Sprintf("perf_doc_%03d", i),
			CollectionID: "default",
			Title:        fmt.Sprintf("Generated %03d", i),
			Body:         body,
			Message:      "generated dataset smoke",
		})
		if err != nil {
			t.Fatalf("create document %03d: %v", i, err)
		}
		if i == 0 {
			first = doc
		}
		if i == 1 {
			second = doc
		}
	}
	t.Logf("created %d generated documents in %s", documentCount, time.Since(started))
	if err := st.RebuildDocumentLinks(ctx, first.ID); err != nil {
		t.Fatalf("rebuild first document links: %v", err)
	}

	result, err := st.Search(ctx, SearchRequest{CollectionID: "default", Query: "needle042", Limit: 10})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Hits) != 1 || result.Hits[0].ID != "perf_doc_042" {
		t.Fatalf("search returned unexpected hits: %+v", result.Hits)
	}

	links, err := st.ListDocumentLinks(ctx, first.ID, "outgoing")
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links.Outgoing) != 1 || links.Outgoing[0].TargetDocumentID != second.ID {
		t.Fatalf("outgoing link was not resolved: %+v", links.Outgoing)
	}

	graph, err := st.Graph(ctx, GraphRequest{Roots: []string{first.ID}, Direction: "outgoing", Depth: 1, MaxNodes: 10, MaxEdges: 10})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if len(graph.Nodes) < 2 || len(graph.Edges) < 1 {
		t.Fatalf("graph did not include expected linked nodes/edges: %+v", graph)
	}

	res, err := st.CreateResource(ctx, CreateResourceRequest{
		CollectionID: "default",
		Filename:     "dataset.txt",
		MIMEType:     "text/plain",
		Content:      strings.NewReader("generated dataset resource"),
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{DocumentID: first.ID, ResourceID: res.ID, RelationType: "attachment"}); err != nil {
		t.Fatalf("attach resource: %v", err)
	}
	refs, err := st.ListDocumentResources(ctx, first.ID)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if len(refs) != 1 || refs[0].ResourceID != res.ID {
		t.Fatalf("resource reference mismatch: %+v", refs)
	}
}

func BenchmarkGeneratedDatasetSearch(b *testing.B) {
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", b.TempDir())
	if err != nil {
		b.Fatalf("open sqlite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		b.Fatalf("bootstrap: %v", err)
	}
	for i := 0; i < 1000; i++ {
		_, err := st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID:  fmt.Sprintf("bench_doc_%04d", i),
			CollectionID: "default",
			Title:        fmt.Sprintf("Benchmark Document %04d", i),
			Body:         fmt.Sprintf("# Benchmark %04d\n\ncommon benchmark token group%d unique%d.\n", i, i%10, i),
			Message:      "benchmark setup",
		})
		if err != nil {
			b.Fatalf("create document %04d: %v", i, err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.Search(ctx, SearchRequest{CollectionID: "default", Query: "benchmark group5", Limit: 20}); err != nil {
			b.Fatalf("search: %v", err)
		}
	}
}
