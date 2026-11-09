module github.com/looplj/axonhub/llm

go 1.26.4

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.26
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/go-cmp v0.7.0
	github.com/google/jsonschema-go v0.4.3
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/kaptinlin/jsonrepair v0.4.10
	github.com/klauspost/compress v1.20.3
	github.com/samber/lo v1.53.0
	github.com/spf13/cast v1.10.0
	github.com/stretchr/testify v1.12.1
	github.com/tidwall/gjson v1.20.0
	github.com/tidwall/sjson v1.2.5
	github.com/tmaxmax/go-sse v0.11.0
	golang.org/x/oauth2 v0.37.0
	golang.org/x/sync v0.24.0
	google.golang.org/api v0.311.0
)

require (
	cloud.google.com/go/auth v0.24.1-0.20261001053825-dbc26066f70a // indirect
	cloud.google.com/go/auth/oauth2adapt v0.3.1 // indirect
	cloud.google.com/go/compute/metadata v0.10.0 // indirect
	github.com/aws/smithy-go v1.28.8 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-json-experiment/json v0.0.0-20260601182631-00ed12fed2a6 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/s2a-go v0.1.11 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.24 // indirect
	github.com/googleapis/gax-go/v2 v2.27.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.67.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.69.0 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	golang.org/x/vuln v1.9.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261026165753-2ffab32bce7e // indirect
	google.golang.org/grpc v1.85.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/tmaxmax/go-sse => github.com/wtj-0527/go-sse v0.0.0-20260811060543-0bb36b8ea0cd

tool golang.org/x/vuln/cmd/govulncheck
