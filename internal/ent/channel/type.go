package channel

import (
	"strings"
)

func (t Type) IsAnthropic() bool {
	return t == TypeAnthropic
}

func (t Type) IsAnthropicLike() bool {
	return strings.HasSuffix(string(t), "_anthropic")
}

func (t Type) IsOpenAI() bool {
	return !t.IsAnthropicLike() && !t.IsAnthropic()
}
