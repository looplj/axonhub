package llm

import (
	"encoding/json"
	"fmt"
)

// ModerationInput 表示 /v1/moderations 的 input 字段。
// 支持 string、[]string，以及多模态对象数组。
type ModerationInput struct {
	String      string                     `json:"-"`
	StringArray []string                   `json:"-"`
	Parts       []ModerationMultiModalPart `json:"-"`
}

// ModerationMultiModalPart 是 omni-moderation 多模态输入的一项。
type ModerationMultiModalPart struct {
	Type     string                    `json:"type"`
	Text     string                    `json:"text,omitempty"`
	ImageURL *ModerationImageURLObject `json:"image_url,omitempty"`
}

// ModerationImageURLObject 包含图片 URL 或 base64 data URL。
type ModerationImageURLObject struct {
	URL string `json:"url"`
}

func (m ModerationInput) MarshalJSON() ([]byte, error) {
	if len(m.Parts) > 0 {
		return json.Marshal(m.Parts)
	}

	if len(m.StringArray) > 0 {
		return json.Marshal(m.StringArray)
	}

	if m.String != "" {
		return json.Marshal(m.String)
	}

	return json.Marshal(nil)
}

func (m *ModerationInput) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("moderation input is nil")
	}

	*m = ModerationInput{}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		m.String = str
		return nil
	}

	// 先尝试字符串数组，再尝试多模态对象数组，避免对象被误解析。
	var strArray []string
	if err := json.Unmarshal(data, &strArray); err == nil {
		// 空数组也是合法的解析结果，后续由校验层拒绝。
		m.StringArray = strArray
		return nil
	}

	var parts []ModerationMultiModalPart
	if err := json.Unmarshal(data, &parts); err == nil {
		m.Parts = parts
		return nil
	}

	return fmt.Errorf("invalid moderation input type")
}

// ModerationRequest 是独立 /v1/moderations 请求体（Model 在父 Request 上）。
type ModerationRequest struct {
	Input ModerationInput `json:"input"`
}

// ModerationClassification 是单条审核分类结果。
type ModerationClassification struct {
	Flagged                   bool                `json:"flagged"`
	Categories                map[string]bool     `json:"categories"`
	CategoryScores            map[string]float64  `json:"category_scores"`
	CategoryAppliedInputTypes map[string][]string `json:"category_applied_input_types,omitempty"`
}

// ModerationResponse 是独立 /v1/moderations 响应（Usage 等公共字段在父 Response 上）。
type ModerationResponse struct {
	Results []ModerationClassification `json:"results"`
}
