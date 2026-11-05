package responses

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestFunctionToolStrictDefaults(t *testing.T) {
	// Monitor accepts either command or ws, not both. Strict normalization must
	// not turn these optional alternatives into required fields.
	const schema = `{
		"type":"object",
		"properties":{
			"description":{"type":"string"},
			"command":{"type":"string"},
			"ws":{
				"type":"object",
				"properties":{
					"url":{"type":"string"},
					"protocols":{"type":"array","items":{"type":"string"}}
				},
				"required":["url"],
				"additionalProperties":false
			}
		},
		"required":["description"],
		"additionalProperties":false
	}`

	for _, source := range []struct {
		name         string
		inbound      transformer.Inbound
		request      string
		defaultFalse bool
	}{
		{
			name:         "anthropic function",
			inbound:      anthropic.NewInboundTransformer(),
			request:      `{"model":"test","max_tokens":1024,"messages":[{"role":"user","content":"Watch the build"}],"tools":[{"name":"Monitor","input_schema":` + schema + `%s}]}`,
			defaultFalse: true,
		},
		{
			name:         "anthropic custom",
			inbound:      anthropic.NewInboundTransformer(),
			request:      `{"model":"test","max_tokens":1024,"messages":[{"role":"user","content":"Watch the build"}],"tools":[{"type":"custom","name":"Monitor","input_schema":` + schema + `%s}]}`,
			defaultFalse: true,
		},
		{
			name:    "native responses",
			inbound: NewInboundTransformer(),
			request: `{"model":"test","input":"Watch the build","tools":[{"type":"function","name":"Monitor","parameters":` + schema + `%s}]}`,
		},
	} {
		for _, strict := range []struct {
			name  string
			field string
			value bool
		}{
			{name: "omitted"},
			{name: "false", field: `,"strict":false`},
			{name: "true", field: `,"strict":true`, value: true},
		} {
			t.Run(source.name+"/"+strict.name, func(t *testing.T) {
				in, err := source.inbound.TransformRequest(t.Context(), &httpclient.Request{
					Headers: http.Header{"Content-Type": []string{"application/json"}},
					Body:    []byte(fmt.Sprintf(source.request, strict.field)),
				})
				require.NoError(t, err)
				originalTools, err := json.Marshal(in.Tools)
				require.NoError(t, err)

				out, err := NewOutboundTransformer("https://example.com", "test-key")
				require.NoError(t, err)
				wire, err := out.TransformRequest(t.Context(), in)
				require.NoError(t, err)
				tools := gjson.GetBytes(wire.Body, "tools").Array()
				require.Len(t, tools, 1)
				require.Equal(t, "function", tools[0].Get("type").String())
				require.Equal(t, "Monitor", tools[0].Get("name").String())
				gotStrict := tools[0].Get("strict")
				if source.defaultFalse || strict.field != "" {
					require.True(t, gotStrict.Exists(), "strict must be explicit on the outbound wire")
					require.Equal(t, fmt.Sprint(strict.value), gotStrict.Raw)
				} else {
					require.False(t, gotStrict.Exists(), "native Responses must retain its default")
				}
				if !strict.value {
					require.JSONEq(t, schema, tools[0].Get("parameters").Raw,
						"optional fields, including nested fields, must remain optional")
				}

				afterTools, err := json.Marshal(in.Tools)
				require.NoError(t, err)
				require.JSONEq(t, string(originalTools), string(afterTools), "shared tools must not be mutated")
			})
		}
	}
}
