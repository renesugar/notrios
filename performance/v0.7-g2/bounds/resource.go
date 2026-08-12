package bounds

import "fmt"

const ChunkDescriptorBytes = 96

type ResourcePolicy struct {
	WholeBelow int64
	ChunkBytes int64
}

func ProposedResourcePolicy() ResourcePolicy {
	return ResourcePolicy{WholeBelow: 1 << 20, ChunkBytes: 1 << 20}
}

type ResourceEstimate struct {
	SizeBytes                   int64 `json:"size_bytes"`
	Objects                     int64 `json:"objects"`
	ManifestBytes               int64 `json:"manifest_bytes"`
	MaximumRetryBytes           int64 `json:"maximum_retry_bytes"`
	RangeRequestBytes           int64 `json:"range_request_bytes"`
	RangeTransferBytes          int64 `json:"range_transfer_bytes"`
	WorstCaseRangeTransferBytes int64 `json:"worst_case_range_transfer_bytes"`
}

func EstimateResource(size, requestedRange int64, policy ResourcePolicy) (ResourceEstimate, error) {
	if size < 0 || requestedRange < 0 || policy.WholeBelow <= 0 || policy.ChunkBytes <= 0 {
		return ResourceEstimate{}, fmt.Errorf("resource policy and sizes must be non-negative")
	}
	result := ResourceEstimate{SizeBytes: size, RangeRequestBytes: min(size, requestedRange)}
	if size == 0 {
		result.Objects = 1
		return result, nil
	}
	if size < policy.WholeBelow {
		result.Objects = 1
		result.MaximumRetryBytes = size
		result.RangeTransferBytes = size
		result.WorstCaseRangeTransferBytes = size
		return result, nil
	}
	result.Objects = (size + policy.ChunkBytes - 1) / policy.ChunkBytes
	result.ManifestBytes = result.Objects * ChunkDescriptorBytes
	result.MaximumRetryBytes = min(size, policy.ChunkBytes)
	if requestedRange > 0 {
		chunks := (min(size, requestedRange) + policy.ChunkBytes - 1) / policy.ChunkBytes
		result.RangeTransferBytes = min(size, chunks*policy.ChunkBytes)
		worstChunks := (min(size, requestedRange) + 2*policy.ChunkBytes - 2) / policy.ChunkBytes
		result.WorstCaseRangeTransferBytes = min(size, worstChunks*policy.ChunkBytes)
	}
	return result, nil
}

type Distribution struct {
	Count      int64 `json:"count"`
	BytesTotal int64 `json:"bytes_total"`
	BytesP50   int64 `json:"bytes_p50"`
	BytesP90   int64 `json:"bytes_p90"`
	BytesP95   int64 `json:"bytes_p95"`
	BytesP99   int64 `json:"bytes_p99"`
	BytesMax   int64 `json:"bytes_max"`
}

// EstimateDistribution uses five aggregate order statistics as an explicit
// proxy sample. It does not pretend to reconstruct private per-file sizes.
func EstimateDistribution(input Distribution, policy ResourcePolicy) map[string]int64 {
	sample := []int64{input.BytesP50, input.BytesP90, input.BytesP95, input.BytesP99, input.BytesMax}
	var objects, manifest, maximumRetry int64
	for _, size := range sample {
		estimate, _ := EstimateResource(size, 64<<10, policy)
		objects += estimate.Objects
		manifest += estimate.ManifestBytes
		if estimate.MaximumRetryBytes > maximumRetry {
			maximumRetry = estimate.MaximumRetryBytes
		}
	}
	return map[string]int64{
		"proxy_sample_points":  int64(len(sample)),
		"proxy_objects":        objects,
		"proxy_manifest_bytes": manifest,
		"maximum_retry_bytes":  maximumRetry,
	}
}
