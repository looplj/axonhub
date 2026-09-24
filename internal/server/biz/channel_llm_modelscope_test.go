package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/modelscope"
)

// TestBuildChannelWithOutbounds_ModelScopeBindsImageEndpoint covers the real
// channel build path: the default ModelScope image endpoint must resolve to the
// ModelScope transformer so image requests never fall back to the OpenAI one.
func TestBuildChannelWithOutbounds_ModelScopeBindsImageEndpoint(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:modelscope_outbounds?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	svc := NewChannelServiceForTest(client)

	built, err := svc.buildChannelWithOutbounds(&ent.Channel{
		ID:          1,
		Name:        "modelscope",
		Type:        channel.TypeModelscope,
		BaseURL:     "https://api-inference.modelscope.cn/v1",
		Credentials: objects.ChannelCredentials{APIKey: "test-key"},
	})
	require.NoError(t, err)

	imageOutbound := built.Outbounds[llm.APIFormatModelScopeImage.String()]
	require.IsType(t, &modelscope.OutboundTransformer{}, imageOutbound)

	// The chat endpoint stays on the same provider transformer, which keeps
	// pass-through enabled for chat because both sides use openai/chat_completions.
	chatOutbound := built.Outbounds[llm.APIFormatOpenAIChatCompletion.String()]
	require.Same(t, built.Outbound, chatOutbound)
}
