// 职责: 定义官方 OpenAI Realtime 图像和缓存交集价格以及完整性校验。
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
func ValidateOpenAIRealtimePrice(price ModelPrice) error {
	if err := ValidateRealtimePrice(price); err != nil {
		return err
	}
	validate := func(items []ModelPriceItem) error {
		seen := map[PriceItemCode]bool{}
		for _, item := range items {
			seen[item.ItemCode] = true
		}
		for _, code := range AllRealtimePriceItemCodes() {
			if !seen[code] {
				return fmt.Errorf("missing OpenAI realtime price %q", code)
			}
		}
		return nil
	}
	if err := validate(price.Items); err != nil {
		return err
	}
	if price.Schedule != nil {
		for _, override := range price.Schedule.Overrides {
			if err := validate(override.Items); err != nil {
				return err
			}
		}
	}
	return nil
}
