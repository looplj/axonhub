package biz

var defaultStoragePolicy = StoragePolicy{
	StoreChunks:       false,
	LivePreview:       false,
	StoreRequestBody:  true,
	StoreResponseBody: true,
	CleanupOptions: []CleanupOption{
		{
			ResourceType: "requests",
			Enabled:      false,
			CleanupDays:  3,
		},
		{
			ResourceType: "usage_logs",
			Enabled:      false,
			CleanupDays:  30,
		},
	},
}

var defaultRetryPolicy = RetryPolicy{
	MaxChannelRetries:       3,
	MaxSingleChannelRetries: 2,
	RetryDelayMs:            1000,
	LoadBalancerStrategy:    "adaptive",
	Enabled:                 true,
}

var defaultModelSettings = SystemModelSettings{
	FallbackToChannelsOnModelNotFound: true,
	QueryAllChannelModels:             true,
	DefaultModelAPIIncludeAll:         false,
}

var defaultChannelSetting = SystemChannelSettings{
	Probe: ChannelProbeSetting{
		Enabled:   true,
		Frequency: ProbeFrequency5Min,
	},
	AutoSync: ChannelModelAutoSyncSetting{
		Frequency: AutoSyncFrequencyOneHour,
	},
}

var defaultGeneralSettings = SystemGeneralSettings{
	CurrencyCode: "USD",
	Timezone:     "UTC",
}

var defaultAutoBackupSettings = AutoBackupSettings{
	Enabled:            false,
	Frequency:          BackupFrequencyDaily,
	IncludeChannels:    true,
	IncludeModels:      true,
	IncludeAPIKeys:     false,
	IncludeModelPrices: true,
	RetentionDays:      30,
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
