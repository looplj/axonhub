package conf

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCustomizedDecodeHook_ResponsesOnlyDataPolicy(t *testing.T) {
	got, err := customizedDecodeHook(
		reflect.TypeFor[string](),
		reflect.TypeFor[biz.ResponsesOnlyDataPolicy](),
		string(biz.ResponsesOnlyDataPolicyReject),
	)

	require.NoError(t, err)
	require.Equal(t, biz.ResponsesOnlyDataPolicyReject, got)
}
