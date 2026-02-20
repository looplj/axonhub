module github.com/looplj/axonhub/cmd/axonclaw

go 1.26.0

require (
	github.com/google/jsonschema-go v0.4.2
	github.com/google/uuid v1.6.0
	github.com/looplj/axonhub/axon v0.0.0
)

require (
	github.com/anthropics/anthropic-sdk-go v1.22.1 // indirect
	github.com/looplj/skills v0.0.0 // indirect
	github.com/samber/lo v1.52.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/text v0.28.0 // indirect
)

replace github.com/looplj/axonhub/axon => ../../axon

replace github.com/looplj/skills => ../../../../../skills
