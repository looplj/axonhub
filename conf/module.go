package conf

import "go.uber.org/fx"

// Module provides the startup loader. Runtime reload is intentionally wired
// by the application for each supported configuration section.
var Module = fx.Module("conf", fx.Provide(NewLoader))
