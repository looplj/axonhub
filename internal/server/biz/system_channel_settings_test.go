package biz

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSystemChannelSettingsPrompts(t *testing.T) {
	setting := SystemChannelSettings{TestSystemPrompt: " \n", TestUserPrompt: "自定义"}
	normalizeSystemChannelSettings(&setting)

	require.Equal(t, defaultChannelTestSystemPrompt, setting.TestSystemPrompt)
	require.Equal(t, "自定义", setting.TestUserPrompt)
	require.NoError(t, validateSystemChannelSettings(&setting))
}

func TestValidateSystemChannelSettingsPromptLength(t *testing.T) {
	setting := SystemChannelSettings{
		TestSystemPrompt: strings.Repeat("a", maxChannelTestPromptRunes+1),
		TestUserPrompt:   defaultChannelTestUserPrompt,
	}

	require.Error(t, validateSystemChannelSettings(&setting))
}
