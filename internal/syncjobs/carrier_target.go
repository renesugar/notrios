package syncjobs

import (
	"context"
	"fmt"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/synccarrier"
)

// CarrierBuild creates a fresh round and provider for one attempt. The builder
// owns all sensitive configuration. The returned budget carrier must wrap the
// carrier used by both round and provider so one allowance covers pull, push,
// and requested resource materialization.
type CarrierBuild func(options synccarrier.Options, byteBudget int64) (*synccarrier.Round, *synccarrier.BudgetCarrier, store.ObjectProvider, error)

type CarrierTarget struct {
	Store            *store.SQLiteStore
	Build            CarrierBuild
	MaterializeLimit int
	// Catchup is optional because directory and REST adapters acquire physical
	// snapshots differently. Local UI/CLI may enqueue this kind; MCP and the
	// ordinary G15 REST start surface still cannot.
	Catchup func(ctx context.Context, byteBudget int64, progress Progress) (map[string]any, error)
}

func (t *CarrierTarget) Plan(ctx context.Context, kind string, byteBudget int64) (map[string]any, error) {
	if t.Store == nil || t.Build == nil {
		return nil, fmt.Errorf("sync target is not wired")
	}
	vector, err := t.Store.SyncStateVector(ctx)
	if err != nil {
		return nil, err
	}
	wanted, err := synccarrier.NewStoreReplica(t.Store).WantedSyncObjects(ctx, 64)
	if err != nil {
		return nil, err
	}
	phases := []string{"plan", "pull", "resource_serve", "push", "advertise"}
	if kind == store.JobKindSyncResourceFetch {
		phases = append(phases, "resource_fetch")
	}
	return map[string]any{
		"state_vector_entries": len(vector),
		"wanted_resources":     len(wanted),
		"phases":               phases,
	}, nil
}

func (t *CarrierTarget) Run(ctx context.Context, kind string, byteBudget int64, progress Progress) (map[string]any, error) {
	if t.Store == nil || t.Build == nil {
		return nil, fmt.Errorf("sync target is not wired")
	}
	if kind == store.JobKindSyncCatchup {
		if t.Catchup == nil {
			return nil, fmt.Errorf("catch-up is not available for this configured target")
		}
		return t.Catchup(ctx, byteBudget, progress)
	}
	if kind != store.JobKindSyncIncremental && kind != store.JobKindSyncResourceFetch {
		return nil, fmt.Errorf("sync job kind %q is not implemented by the carrier target", kind)
	}
	var budget *synccarrier.BudgetCarrier
	options := synccarrier.Options{Checkpoint: func(ctx context.Context, phase string, result synccarrier.Result) error {
		used := int64(0)
		if budget != nil {
			used = budget.Used()
		}
		processed := int64(result.AdmittedOperations + result.PublishedOperations)
		return progress(phase, processed, 0, used, map[string]any{
			"last_phase": phase, "admitted_operations": result.AdmittedOperations,
			"published_operations": result.PublishedOperations,
		})
	}}
	round, builtBudget, provider, err := t.Build(options, byteBudget)
	if err != nil {
		return nil, err
	}
	budget = builtBudget
	result, err := round.Run(ctx)
	if err != nil {
		return nil, err
	}
	summary := map[string]any{
		"peers": len(result.Peers), "admitted_operations": result.AdmittedOperations,
		"published_operations": result.PublishedOperations, "published_objects": result.PublishedObjects,
		"bytes_used": budget.Used(),
	}
	if kind == store.JobKindSyncResourceFetch {
		if provider == nil {
			return nil, fmt.Errorf("resource provider is not wired")
		}
		limit := t.MaterializeLimit
		if limit <= 0 || limit > 64 {
			limit = 16
		}
		materialized, materializeErr := t.Store.MaterializeResources(ctx, provider, limit)
		if materializeErr != nil {
			return nil, materializeErr
		}
		summary["resources_considered"] = materialized.Considered
		summary["resources_materialized"] = materialized.Materialized
		summary["resource_bytes"] = materialized.FetchedBytes
		if err := progress("resource_fetch", int64(materialized.Materialized), int64(materialized.Considered),
			budget.Used(), map[string]any{"last_phase": "resource_fetch", "materialized": materialized.Materialized}); err != nil {
			return nil, err
		}
	}
	return summary, nil
}

func (t *CarrierTarget) Discover(ctx context.Context) (map[string]any, error) {
	if t.Store == nil || t.Build == nil {
		return nil, fmt.Errorf("sync target is not wired")
	}
	round, _, _, err := t.Build(synccarrier.Options{}, store.DefaultSyncJobByteBudget)
	if err != nil {
		return nil, err
	}
	result, err := round.Discover(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"namespaces": result.Namespaces, "peers": result.Peers,
		"candidates": result.Candidates, "skipped": result.Skipped,
		"scanned_artifacts": result.ScannedArtifacts,
	}, nil
}
