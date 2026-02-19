package cc

import (
	"context"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

const (
	billingHeaderPrefix = "x-anthropic-billing-header:"
	BillingCCHKey       = "claudecode_billing_cch"
)

// StripBillingHeaderCCH removes the random `cch=...;` suffix from Claude Code's
// "x-anthropic-billing-header:" system message.
//
// It runs on unified llm.Request (OnInboundLlmRequest) so the transformation is applied
// consistently regardless of provider/channel retries.
//
// The stripped cch value (if any) is saved into request.TransformerMetadata[BillingCCHKey]
// for restoration by Claude Code outbound transformer when using official credentials.
func StripBillingHeaderCCH() pipeline.Middleware {
	return pipeline.OnLlmRequest("claudecode-strip-billing-cch", func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
		if request == nil || len(request.Messages) == 0 {
			return request, nil
		}

		var capturedCCH string
		changed := false

		for i := range request.Messages {
			msg := &request.Messages[i]
			if msg.Role != "system" {
				continue
			}

			if msg.Content.Content != nil {
				newText, cch, didChange := stripBillingHeaderCCHFromText(*msg.Content.Content)
				if didChange {
					changed = true
					*msg.Content.Content = newText
					if capturedCCH == "" && cch != "" {
						capturedCCH = cch
					}
				}
			}

			if len(msg.Content.MultipleContent) > 0 {
				for j := range msg.Content.MultipleContent {
					part := &msg.Content.MultipleContent[j]
					if part.Type != "text" || part.Text == nil {
						continue
					}

					newText, cch, didChange := stripBillingHeaderCCHFromText(*part.Text)
					if didChange {
						changed = true
						*part.Text = newText
						if capturedCCH == "" && cch != "" {
							capturedCCH = cch
						}
					}
				}
			}
		}

		if !changed {
			return request, nil
		}

		if capturedCCH != "" {
			if request.TransformerMetadata == nil {
				request.TransformerMetadata = make(map[string]any)
			}
			if _, exists := request.TransformerMetadata[BillingCCHKey]; !exists {
				request.TransformerMetadata[BillingCCHKey] = capturedCCH
			}
		}

		return request, nil
	})
}

func stripBillingHeaderCCHFromText(text string) (string, string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return text, "", false
	}

	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, billingHeaderPrefix) {
		return text, "", false
	}

	rest := strings.TrimSpace(trimmed[len(billingHeaderPrefix):])
	if rest == "" {
		return text, "", false
	}

	originalHadTrailingSemi := strings.HasSuffix(strings.TrimSpace(rest), ";")

	parts := strings.Split(rest, ";")
	kept := make([]string, 0, len(parts))
	var cch string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		pl := strings.ToLower(p)
		if strings.HasPrefix(pl, "cch=") {
			if cch == "" {
				cch = strings.TrimSpace(p[len("cch="):])
			}
			continue
		}

		kept = append(kept, p)
	}

	if cch == "" {
		return text, "", false
	}

	out := billingHeaderPrefix + " " + strings.Join(kept, "; ")
	if originalHadTrailingSemi || len(kept) > 0 {
		out = strings.TrimSpace(out) + ";"
	}

	return out, cch, true
}

