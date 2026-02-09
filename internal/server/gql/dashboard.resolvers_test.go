package gql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testStats is a test struct that implements the statsItem constraint
type testStats struct {
	ID           string
	Name         string
	RequestCount int64
	Throughput   float64
}

// TestCalculateConfidenceAndSort_EmptyResults tests behavior with empty input.
func TestCalculateConfidenceAndSort_EmptyResults(t *testing.T) {
	results := []testStats{}
	limit := 5

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	assert.Nil(t, got, "should return nil for empty input")
}

// TestCalculateConfidenceAndSort_SingleItem tests behavior with single item.
func TestCalculateConfidenceAndSort_SingleItem(t *testing.T) {
	results := []testStats{
		{ID: "1", Name: "item1", RequestCount: 100, Throughput: 50.0},
	}
	limit := 5

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	require.Len(t, got, 1)
	assert.Equal(t, "1", got[0].stats.ID)
	assert.Equal(t, int64(100), got[0].stats.RequestCount)
	assert.Equal(t, 50.0, got[0].stats.Throughput)
}

// TestCalculateConfidenceAndSort_SortByConfidence tests that high confidence items come first.
func TestCalculateConfidenceAndSort_SortByConfidence(t *testing.T) {
	// Items with varying request counts to produce different confidence levels
	// With median around 100:
	// - 10 requests -> low confidence
	// - 100 requests -> medium confidence
	// - 500 requests -> high confidence
	results := []testStats{
		{ID: "low", Name: "low-confidence", RequestCount: 10, Throughput: 100.0},
		{ID: "high", Name: "high-confidence", RequestCount: 500, Throughput: 80.0},
		{ID: "medium", Name: "medium-confidence", RequestCount: 100, Throughput: 90.0},
	}
	limit := 5

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	require.Len(t, got, 3)
	// High confidence should be first
	assert.Equal(t, "high", got[0].confidence)
	assert.Equal(t, "high", got[0].stats.ID)
	// Medium confidence should be second
	assert.Equal(t, "medium", got[1].confidence)
	assert.Equal(t, "medium", got[1].stats.ID)
	// Low confidence should be last
	assert.Equal(t, "low", got[2].confidence)
	assert.Equal(t, "low", got[2].stats.ID)
}

// TestCalculateConfidenceAndSort_SortByThroughputWithinSameConfidence tests throughput sorting.
func TestCalculateConfidenceAndSort_SortByThroughputWithinSameConfidence(t *testing.T) {
	// All items have same request count (100) so same confidence level (medium)
	// Should be sorted by throughput descending
	results := []testStats{
		{ID: "medium", RequestCount: 100, Throughput: 50.0},
		{ID: "high", RequestCount: 100, Throughput: 100.0},
		{ID: "low", RequestCount: 100, Throughput: 25.0},
	}
	limit := 5

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	require.Len(t, got, 3)
	// Within same confidence, should sort by throughput descending
	assert.Equal(t, "high", got[0].stats.ID)
	assert.Equal(t, 100.0, got[0].stats.Throughput)
	assert.Equal(t, "medium", got[1].stats.ID)
	assert.Equal(t, 50.0, got[1].stats.Throughput)
	assert.Equal(t, "low", got[2].stats.ID)
	assert.Equal(t, 25.0, got[2].stats.Throughput)
}

// TestCalculateConfidenceAndSort_LimitRespected tests that limit is respected.
func TestCalculateConfidenceAndSort_LimitRespected(t *testing.T) {
	results := []testStats{
		{ID: "1", RequestCount: 500, Throughput: 100.0},
		{ID: "2", RequestCount: 400, Throughput: 90.0},
		{ID: "3", RequestCount: 300, Throughput: 80.0},
		{ID: "4", RequestCount: 200, Throughput: 70.0},
		{ID: "5", RequestCount: 100, Throughput: 60.0},
		{ID: "6", RequestCount: 50, Throughput: 50.0},
	}
	limit := 3

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	require.Len(t, got, 3)
	// Should only get top 3 by confidence, then throughput
	assert.Equal(t, "1", got[0].stats.ID)
	assert.Equal(t, "2", got[1].stats.ID)
	assert.Equal(t, "3", got[2].stats.ID)
}

// TestCalculateConfidenceAndSort_FilterLowConfidenceWhenEnoughHighMedium tests filtering.
func TestCalculateConfidenceAndSort_FilterLowConfidenceWhenEnoughHighMedium(t *testing.T) {
	// With limit=2 and 2 high/medium items, low confidence should be filtered out
	results := []testStats{
		{ID: "high1", RequestCount: 500, Throughput: 100.0},
		{ID: "high2", RequestCount: 400, Throughput: 90.0},
		{ID: "low1", RequestCount: 10, Throughput: 200.0}, // High throughput but low confidence
		{ID: "low2", RequestCount: 5, Throughput: 150.0},  // High throughput but low confidence
	}
	limit := 2

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	require.Len(t, got, 2) // Should only get the 2 high confidence items
	assert.Equal(t, "high", got[0].confidence)
	assert.Equal(t, "medium", got[1].confidence)
	// Low confidence items should be filtered out since we have enough high/medium
	for _, item := range got {
		assert.NotEqual(t, "low", item.confidence)
	}
}

// TestCalculateConfidenceAndSort_IncludeLowWhenNotEnoughHighMedium tests fallback.
func TestCalculateConfidenceAndSort_IncludeLowWhenNotEnoughHighMedium(t *testing.T) {
	// With limit=5 and only 2 high/medium items, low confidence should be included
	results := []testStats{
		{ID: "high", RequestCount: 500, Throughput: 100.0},
		{ID: "medium", RequestCount: 100, Throughput: 80.0},
		{ID: "low1", RequestCount: 10, Throughput: 200.0},
		{ID: "low2", RequestCount: 5, Throughput: 150.0},
	}
	limit := 5

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	// Should include all items since we don't have enough high/medium to meet limit
	require.Len(t, got, 4)
	// But low confidence items should still be sorted last
	assert.Equal(t, "low", got[2].confidence)
	assert.Equal(t, "low", got[3].confidence)
}

// TestCalculateConfidenceAndSort_MedianCalculation tests median calculation.
func TestCalculateConfidenceAndSort_MedianCalculation(t *testing.T) {
	// Test with even number of items
	resultsEven := []testStats{
		{ID: "1", RequestCount: 10, Throughput: 50.0},
		{ID: "2", RequestCount: 20, Throughput: 60.0},
		{ID: "3", RequestCount: 30, Throughput: 70.0},
		{ID: "4", RequestCount: 40, Throughput: 80.0},
	}
	// Median for even count: (20+30)/2 = 25

	gotEven := calculateConfidenceAndSort(resultsEven,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		4,
	)

	require.Len(t, gotEven, 4)
	// With median=25, items with request count > 25 should have higher confidence
	// Item 3 (30) and 4 (40) should have medium/high confidence
	// Item 1 (10) and 2 (20) should have low confidence
	assert.True(t, gotEven[0].stats.RequestCount > 25 || gotEven[1].stats.RequestCount > 25)

	// Test with odd number of items
	resultsOdd := []testStats{
		{ID: "1", RequestCount: 10, Throughput: 50.0},
		{ID: "2", RequestCount: 20, Throughput: 60.0},
		{ID: "3", RequestCount: 30, Throughput: 70.0},
	}
	// Median for odd count: 20 (middle value)

	gotOdd := calculateConfidenceAndSort(resultsOdd,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		3,
	)

	require.Len(t, gotOdd, 3)
}

// TestCalculateConfidenceAndSort_PreservesOriginalData tests that original data is preserved.
func TestCalculateConfidenceAndSort_PreservesOriginalData(t *testing.T) {
	results := []testStats{
		{ID: "test-id", Name: "Test Name", RequestCount: 100, Throughput: 50.5},
	}
	limit := 5

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	require.Len(t, got, 1)
	assert.Equal(t, "test-id", got[0].stats.ID)
	assert.Equal(t, "Test Name", got[0].stats.Name)
	assert.Equal(t, int64(100), got[0].stats.RequestCount)
	assert.Equal(t, 50.5, got[0].stats.Throughput)
}

// TestCalculateConfidenceAndSort_ScoreAssignment tests correct score assignment.
func TestCalculateConfidenceAndSort_ScoreAssignment(t *testing.T) {
	// Items with request counts designed to trigger different confidence levels
	// High confidence: >= 500 requests AND >= 1.5x median
	// Medium confidence: >= 100 requests AND >= 0.5x median
	// Low confidence: everything else
	results := []testStats{
		{ID: "high", RequestCount: 1000, Throughput: 100.0}, // Should be high
		{ID: "medium", RequestCount: 200, Throughput: 80.0}, // Should be medium
		{ID: "low", RequestCount: 10, Throughput: 200.0},    // Should be low
	}
	limit := 5

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	require.Len(t, got, 3)

	// Find each item and verify its score
	for _, item := range got {
		switch item.stats.ID {
		case "high":
			assert.Equal(t, "high", item.confidence)
			assert.Equal(t, 3, item.score)
		case "medium":
			assert.Equal(t, "medium", item.confidence)
			assert.Equal(t, 2, item.score)
		case "low":
			assert.Equal(t, "low", item.confidence)
			assert.Equal(t, 1, item.score)
		}
	}
}

// TestCalculateConfidenceAndSort_ZeroLimit tests behavior with zero limit.
func TestCalculateConfidenceAndSort_ZeroLimit(t *testing.T) {
	results := []testStats{
		{ID: "1", RequestCount: 100, Throughput: 50.0},
		{ID: "2", RequestCount: 200, Throughput: 60.0},
	}
	limit := 0

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	// With limit 0, should still process but return empty slice
	assert.Empty(t, got)
}

// TestCalculateConfidenceAndSort_LargeDataset tests performance with larger dataset.
func TestCalculateConfidenceAndSort_LargeDataset(t *testing.T) {
	results := make([]testStats, 100)
	for i := 0; i < 100; i++ {
		results[i] = testStats{
			ID:           string(rune('a' + i%26)),
			RequestCount: int64(i * 10),
			Throughput:   float64(100 - i),
		}
	}
	limit := 10

	got := calculateConfidenceAndSort(results,
		func(item testStats) int64 { return item.RequestCount },
		func(item testStats) float64 { return item.Throughput },
		limit,
	)

	require.Len(t, got, 10)
	// Top items should have highest request counts (and thus highest confidence)
	// Within same confidence, sorted by throughput descending
	for i := 0; i < len(got)-1; i++ {
		// Either confidence is higher, or same confidence with higher throughput
		if got[i].score == got[i+1].score {
			assert.True(t, got[i].stats.Throughput >= got[i+1].stats.Throughput,
				"items with same confidence should be sorted by throughput descending")
		} else {
			assert.True(t, got[i].score > got[i+1].score,
				"items should be sorted by confidence score descending")
		}
	}
}
