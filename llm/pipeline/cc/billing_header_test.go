package cc

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestStripBillingHeaderCCH(t *testing.T) {
	ctx := context.Background()
	mw := StripBillingHeaderCCH()

	t.Run("strips cch from system string content and captures it", func(t *testing.T) {
		billingMsg := "x-anthropic-billing-header: cc_version=2.1.42.c31; cc_entrypoint=cli; cch=38a80;"

		req := &llm.Request{
			Model: "claude-sonnet-4-5",
			Messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: &billingMsg}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
		}

		out, err := mw.OnInboundLlmRequest(ctx, req)
		require.NoError(t, err)

		require.NotNil(t, out)
		require.NotEmpty(t, out.Messages)
		require.NotNil(t, out.Messages[0].Content.Content)

		require.Contains(t, *out.Messages[0].Content.Content, "x-anthropic-billing-header:")
		require.NotContains(t, *out.Messages[0].Content.Content, "cch=")

		require.Equal(t, "x-anthropic-billing-header: cc_version=2.1.42.c31; cc_entrypoint=cli;", *out.Messages[0].Content.Content)

		require.NotNil(t, out.TransformerMetadata)
		require.Equal(t, "38a80", out.TransformerMetadata[BillingCCHKey])
	})

	t.Run("strips cch from system multiple content part", func(t *testing.T) {
		billingMsg := "x-anthropic-billing-header: cc_version=2.1.42.c31; cc_entrypoint=cli; cch=abcde;"

		req := &llm.Request{
			Model: "claude-sonnet-4-5",
			Messages: []llm.Message{
				{
					Role: "system",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{Type: "text", Text: &billingMsg},
						},
					},
				},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
		}

		out, err := mw.OnInboundLlmRequest(ctx, req)
		require.NoError(t, err)

		require.NotNil(t, out.Messages[0].Content.MultipleContent[0].Text)
		require.Contains(t, *out.Messages[0].Content.MultipleContent[0].Text, "x-anthropic-billing-header:")
		require.NotContains(t, *out.Messages[0].Content.MultipleContent[0].Text, "cch=")

		require.NotNil(t, out.TransformerMetadata)
		require.Equal(t, "abcde", out.TransformerMetadata[BillingCCHKey])
	})
}
