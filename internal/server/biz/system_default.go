package biz

const (
	defaultChannelTestSystemPrompt = "You are a helpful assistant."
	defaultChannelTestUserPrompt   = "Hello world, I'm AxonHub.\nPlease tell me who you are?"
	maxChannelTestPromptRunes      = 4096
)

func defaultCleanupOptions() []CleanupOption {
	return []CleanupOption{
		{
			ResourceType: CleanupResourceRequests,
			Enabled:      false,
			CleanupDays:  3,
		},
		{
			ResourceType: CleanupResourceUsageLogs,
			Enabled:      false,
			CleanupDays:  30,
		},
		{
			ResourceType: CleanupResourceRequestBodies,
			Enabled:      false,
			CleanupDays:  7,
		},
		{
			ResourceType: CleanupResourceResponseBodies,
			Enabled:      false,
			CleanupDays:  7,
		},
		{
			ResourceType: CleanupResourceResponseChunks,
			Enabled:      false,
			CleanupDays:  3,
		},
	}
}

// mergeCleanupOptions keeps existing entries and appends any missing defaults.
func mergeCleanupOptions(existing []CleanupOption) []CleanupOption {
	byType := make(map[string]CleanupOption, len(existing))
	order := make([]string, 0, len(existing)+5)

	for _, opt := range existing {
		if _, seen := byType[opt.ResourceType]; !seen {
			order = append(order, opt.ResourceType)
		}

		byType[opt.ResourceType] = opt
	}

	for _, def := range defaultCleanupOptions() {
		if _, seen := byType[def.ResourceType]; seen {
			continue
		}

		byType[def.ResourceType] = def
		order = append(order, def.ResourceType)
	}

	merged := make([]CleanupOption, 0, len(order))
	for _, resourceType := range order {
		merged = append(merged, byType[resourceType])
	}

	return merged
}

var defaultStoragePolicy = StoragePolicy{
	StoreChunks:       false,
	LivePreview:       false,
	StoreRequestBody:  true,
	StoreResponseBody: true,
	CleanupOptions:    defaultCleanupOptions(),
}

var defaultRetryPolicy = RetryPolicy{
	MaxChannelRetries:       3,
	MaxSingleChannelRetries: 2,
	RetryDelayMs:            1000,
	LoadBalancerStrategy:    "adaptive",
	TraceStickyMode:         TraceStickyPreferPreviousChannel,
	Enabled:                 true,
	UpstreamErrorPolicy: UpstreamErrorPolicy{
		Mode: UpstreamErrorModePassthrough,
	},
}

var defaultModelSettings = SystemModelSettings{
	FallbackToChannelsOnModelNotFound: true,
	QueryAllChannelModels:             true,
	DefaultModelAPIIncludeAll:         false,
	AutoReasoningEffort:               false,
	ModelBlacklistRegex:               "",
	HideUnroutableModelsInList:        false,
	DeveloperSettings:                 []*DeveloperModelSettings{},
}

var defaultChannelSetting = SystemChannelSettings{
	Probe: ChannelProbeSetting{
		Enabled:   true,
		Frequency: ProbeFrequency5Min,
	},
	AutoSync: ChannelModelAutoSyncSetting{
		Frequency: AutoSyncFrequencyOneHour,
	},
	TestSystemPrompt: defaultChannelTestSystemPrompt,
	TestUserPrompt:   defaultChannelTestUserPrompt,
}

var defaultGeneralSettings = SystemGeneralSettings{
	CurrencyCode: "USD",
	Timezone:     "UTC",
}

var defaultAutoBackupSettings = AutoBackupSettings{
	Enabled:              false,
	Frequency:            BackupFrequencyDaily,
	IncludeSystemConfigs: false,
	IncludeChannels:      true,
	IncludeModels:        true,
	IncludeAPIKeys:       false,
	IncludeModelPrices:   true,
	IncludeUsageStats:    false,
	IncludeRequestLogs:   false,
	RetentionDays:        30,
}

var defaultVideoStorageSettings = VideoStorageSettings{
	Enabled:             false,
	DataStorageID:       0,
	ScanIntervalMinutes: 1,
	ScanLimit:           50,
}

var defaultQuotaEnforcementSettings = QuotaEnforcementSettings{
	Enabled: false,
	Mode:    QuotaEnforcementModeExhaustedOnly,
}

var defaultSecuritySettings = SecuritySettings{
	BlockedIPs:              []string{},
	ShowRequestLogIPBanIcon: true,
}
