package llm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestResponseMetaAccountingContract(t *testing.T) {
	// Given
	finalZero := llm.ResponseMeta{
		Usage:              &llm.Usage{},
		AccountingEvidence: llm.AccountingEvidenceExplicitFinal,
	}
	partialNonzero := llm.ResponseMeta{Usage: &llm.Usage{TotalTokens: 34}}
	cleanCandidate := llm.ResponseMeta{AccountingEvidence: llm.AccountingEvidenceCleanEndCandidate}
	// When / Then
	require.True(t, finalZero.AccountingFinal(llm.SourceEndUnknown))
	require.False(t, partialNonzero.AccountingFinal(llm.SourceEndUnknown))
	require.True(t, cleanCandidate.AccountingFinal(llm.SourceEndSemantic))
	require.False(t, cleanCandidate.AccountingFinal(llm.SourceEndError))
}
