package llm_test

import (
	"math"
	"testing"
)

type goldenQuery struct {
	Name        string   `json:"name"`
	Query       string   `json:"query"`
	RelevantIDs []string `json:"relevant_ids"`
}

type rankedResult struct {
	Query        goldenQuery
	RetrievedIDs []string
}

type retrievalMetrics struct {
	HitAt1    float64
	RecallAtK float64
	MRRAtK    float64
}

func calculateRetrievalMetrics(results []rankedResult, k int) retrievalMetrics {
	if len(results) == 0 || k <= 0 {
		return retrievalMetrics{}
	}

	var hitAt1Total, recallAtKTotal, reciprocalRankTotal float64
	for _, result := range results {
		relevant := make(map[string]struct{}, len(result.Query.RelevantIDs))
		for _, id := range result.Query.RelevantIDs {
			relevant[id] = struct{}{}
		}

		limit := min(k, len(result.RetrievedIDs))
		foundRelevant := 0
		firstRelevantRank := 0
		for i, id := range result.RetrievedIDs[:limit] {
			if _, ok := relevant[id]; !ok {
				continue
			}
			foundRelevant++
			if firstRelevantRank == 0 {
				firstRelevantRank = i + 1
			}
		}

		if limit > 0 {
			if _, ok := relevant[result.RetrievedIDs[0]]; ok {
				hitAt1Total++
			}
		}
		if len(relevant) > 0 {
			recallAtKTotal += float64(foundRelevant) / float64(len(relevant))
		}
		if firstRelevantRank > 0 {
			reciprocalRankTotal += 1 / float64(firstRelevantRank)
		}
	}

	count := float64(len(results))
	return retrievalMetrics{
		HitAt1:    hitAt1Total / count,
		RecallAtK: recallAtKTotal / count,
		MRRAtK:    reciprocalRankTotal / count,
	}
}

func TestCalculateRetrievalMetrics(t *testing.T) {
	results := []rankedResult{
		{
			Query:        goldenQuery{RelevantIDs: []string{"pasta", "noodles"}},
			RetrievedIDs: []string{"curry", "pasta", "noodles"},
		},
		{
			Query:        goldenQuery{RelevantIDs: []string{"salad"}},
			RetrievedIDs: []string{"salad", "soup", "pasta"},
		},
	}

	got := calculateRetrievalMetrics(results, 3)
	assertMetric(t, "Hit@1", got.HitAt1, 0.5)
	assertMetric(t, "Recall@3", got.RecallAtK, 1.0)
	assertMetric(t, "MRR@3", got.MRRAtK, 0.75)
}

func assertMetric(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Errorf("%s = %.4f, want %.4f", name, got, want)
	}
}
