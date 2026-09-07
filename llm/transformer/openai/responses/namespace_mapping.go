package responses

import (
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

const NamespaceToolMappingMetadataKey = shared.NamespaceToolMappingMetadataKey

// NamespaceToolReference and NamespaceToolMapping are re-exported from the llm
// package so that transformer code can use the short alias without importing
// the pipeline or shared packages.
type NamespaceToolReference = llm.NamespaceToolReference
type NamespaceToolMapping = llm.NamespaceToolMapping

// flatNamesForNamespace returns the flat function names that belong to the
// given namespace. Callers should sort before comparing or when deterministic
// output is required.
func flatNamesForNamespace(mapping NamespaceToolMapping, namespace string) []string {
	if mapping == nil || namespace == "" {
		return nil
	}

	var out []string
	for flat, ref := range mapping {
		if ref.Namespace == namespace {
			out = append(out, flat)
		}
	}

	return out
}
