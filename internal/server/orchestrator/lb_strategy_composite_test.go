package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCompositeStrategy_Score(t *testing.T) {
	ctx := context.Background()

	s1 := &mockStrategy{name: "s1", score: 100}
	s2 := &mockStrategy{name: "s2", score: 50}

	composite := NewCompositeStrategy(s1, s2)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := composite.Score(ctx, channel)
	assert.Equal(t, 150.0, score, "Should sum all strategy scores with default weights")
}

func TestCompositeStrategy_WithWeights(t *testing.T) {
	ctx := context.Background()

	s1 := &mockStrategy{name: "s1", score: 100}
	s2 := &mockStrategy{name: "s2", score: 50}

	composite := NewCompositeStrategy(s1, s2).WithWeights(2.0, 0.5)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := composite.Score(ctx, channel)
	// (100 * 2.0) + (50 * 0.5) = 200 + 25 = 225
	assert.Equal(t, 225.0, score, "Should apply weights to strategy scores")
}

func TestCompositeStrategy_Name(t *testing.T) {
	composite := NewCompositeStrategy()
	assert.Equal(t, "Composite", composite.Name())
}

func TestCompositeStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	s1 := &mockStrategy{name: "s1", score: 100}
	s2 := &mockStrategy{name: "s2", score: 50}

	testCases := []struct {
		name    string
		weights []float64
	}{
		{
			name:    "default weights",
			weights: nil,
		},
		{
			name:    "custom weights",
			weights: []float64{2.0, 0.5},
		},
		{
			name:    "zero weights",
			weights: []float64{0, 0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			composite := NewCompositeStrategy(s1, s2)
			if tc.weights != nil {
				composite = composite.WithWeights(tc.weights...)
			}

			channel := &biz.Channel{
				Channel: &ent.Channel{ID: 1, Name: "test"},
			}

			score := composite.Score(ctx, channel)
			debugScore, _ := composite.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for %s", tc.name)
		})
	}
}
