// 职责: 定义 OpenAI Realtime 图像和缓存费率码，并复用 Realtime 分项价格校验。
// 关联文件:
//
//	internal/objects/price.go
//	internal/server/biz/realtime_billing.go
//	llm/model.go

package objects

import "fmt"

const (
	PriceItemCodeRealtimeInputImage       PriceItemCode = "realtime_input_image_tokens"
	PriceItemCodeRealtimeCachedInputText  PriceItemCode = "realtime_cached_input_text_tokens"
	PriceItemCodeRealtimeCachedInputAudio PriceItemCode = "realtime_cached_input_audio_tokens"
	PriceItemCodeRealtimeCachedInputImage PriceItemCode = "realtime_cached_input_image_tokens"
)

func AllRealtimePriceItemCodes() []PriceItemCode {
	return append(RealtimePriceItemCodes(), PriceItemCodeRealtimeInputImage, PriceItemCodeRealtimeCachedInputText, PriceItemCodeRealtimeCachedInputAudio, PriceItemCodeRealtimeCachedInputImage)
}

// ValidateOpenAIRealtimePrice validates configured OpenAI Realtime rates.
// Voice rates may be partial; the image/cache rate group must be complete.
func ValidateOpenAIRealtimePrice(price ModelPrice) error {
	if err := ValidateRealtimePrice(price); err != nil {
		return err
	}
	// Voice/audio rates may be partial. Preserve the existing completeness
	// requirement for all OpenAI image and cached-input rates.
	validateImageCacheRates := func(label string, items []ModelPriceItem) error {
		seen := make(map[PriceItemCode]struct{}, 4)
		for _, item := range items {
			seen[item.ItemCode] = struct{}{}
		}
		for _, code := range []PriceItemCode{
			PriceItemCodeRealtimeInputImage,
			PriceItemCodeRealtimeCachedInputText,
			PriceItemCodeRealtimeCachedInputAudio,
			PriceItemCodeRealtimeCachedInputImage,
		} {
			if _, ok := seen[code]; !ok {
				return fmt.Errorf("%s: missing OpenAI realtime image/cache price %q", label, code)
			}
		}
		return nil
	}

	if err := validateImageCacheRates("items", price.Items); err != nil {
		return err
	}
	if price.Schedule != nil {
		for i := range price.Schedule.Overrides {
			label := fmt.Sprintf("schedule.overrides[%d].items", i)
			if err := validateImageCacheRates(label, price.Schedule.Overrides[i].Items); err != nil {
				return err
			}
		}
	}
	return nil
}
