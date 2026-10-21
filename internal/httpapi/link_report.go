package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/linkreport"
)

// The ambiguous-links report over REST (v1.0 J40-D, decision 1).
//
// REST carries the report without note content, as it carries lint: a note
// and its candidates by ID, a link by line, column and a SHA-256 of its target.
// What a person needs to choose between candidates -- the link as written and
// every title -- is in `notriosctl links report` and in the Reports note, on
// the owner's machine. There is no MCP tool: a content-free one would be lint's
// `ambiguous_link` check again, and an MCP client is the surface most likely
// to forward what it reads.

func (s *Server) handleLinkReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	req, ok := linkReportRequest(w, r)
	if !ok {
		return
	}
	report, err := linkreport.Generate(r.Context(), s.store, req)
	if writeStoreError(w, err, "link_report_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPILinkReport(report))
}

// handleWriteLinkReportNote regenerates the Reports note. A POST, and never
// anything automatic, because the scan reads the whole library.
func (s *Server) handleWriteLinkReportNote(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	req, ok := linkReportRequest(w, r)
	if !ok {
		return
	}
	doc, report, err := linkreport.WriteNote(r.Context(), s.store, req)
	if writeStoreError(w, err, "link_report_failed") {
		return
	}
	writeJSON(w, http.StatusOK, api.LinkReportNoteWritten{
		DocumentID: doc.ID, URI: doc.URI, Title: doc.Title, Report: toAPILinkReport(report),
	})
}

func linkReportRequest(w http.ResponseWriter, r *http.Request) (linkreport.Request, bool) {
	req := linkreport.Request{CollectionID: strings.TrimSpace(r.URL.Query().Get("collection"))}
	for _, limit := range []struct {
		name    string
		ceiling int
		into    *int
	}{
		{"detail_limit", linkreport.MaxDetailLimit, &req.DetailLimit},
		{"candidate_limit", linkreport.MaxCandidateLimit, &req.CandidateLimit},
	} {
		raw := strings.TrimSpace(r.URL.Query().Get(limit.name))
		if raw == "" {
			continue
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "validation_failed", limit.name+" must be a positive integer")
			return linkreport.Request{}, false
		}
		if parsed > limit.ceiling {
			writeError(w, http.StatusBadRequest, "limit_too_large", limit.name+" is too large")
			return linkreport.Request{}, false
		}
		*limit.into = parsed
	}
	return req, true
}

// toAPILinkReport drops everything a note holds: the link as written, its
// target and every title and URI beyond an ID.
func toAPILinkReport(report linkreport.Report) api.LinkReport {
	out := api.LinkReport{
		Version: report.Version, CollectionID: report.CollectionID,
		DetailLimit: report.DetailLimit, CandidateLimit: report.CandidateLimit,
		NotesWithAmbiguousLinks: report.NotesWithAmbiguousLinks, AmbiguousLinks: report.AmbiguousLinks,
		FoundByLibrary: report.FoundByLibraryCount, FoundByImport: report.FoundByImportCount,
		UnresolvedLinks: report.UnresolvedLinks, Notes: []api.LinkReportNote{},
		Truncated: report.Truncated, ReportSHA256: report.ReportSHA256,
		Warnings: report.Warnings, ElapsedMS: report.ElapsedMS,
	}
	for _, note := range report.Notes {
		entry := api.LinkReportNote{DocumentID: note.DocumentID, LinkCount: note.LinkCount, Links: []api.LinkReportLink{}, Truncated: note.Truncated}
		for _, link := range note.Links {
			sum := sha256.Sum256([]byte(link.RawTarget))
			ids := make([]string, 0, len(link.Candidates))
			for _, candidate := range link.Candidates {
				ids = append(ids, candidate.ID)
			}
			entry.Links = append(entry.Links, api.LinkReportLink{
				Line: link.Line, Column: link.Column, TargetSHA256: hex.EncodeToString(sum[:]),
				FoundBy: link.FoundBy, Kind: link.Kind,
				CandidateCount: link.CandidateCount, CandidateIDs: ids, Truncated: link.Truncated,
			})
		}
		out.Notes = append(out.Notes, entry)
	}
	return out
}
