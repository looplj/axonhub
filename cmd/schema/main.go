package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/looplj/axonhub/conf"
	"go.uber.org/zap/zapcore"
)

func main() {
	r := new(jsonschema.Reflector)
	r.Namer = func(t reflect.Type) string {
		pkg := t.PkgPath()
		if pkg == "" {
			return t.Name()
		}
		parts := strings.Split(pkg, "/")
		return parts[len(parts)-1] + "." + t.Name()
	}
	r.RequiredFromJSONSchemaTags = true

	// Map time.Duration to string with a duration pattern
	r.Mapper = func(t reflect.Type) *jsonschema.Schema {
		if t == reflect.TypeFor[time.Duration]() {
			return &jsonschema.Schema{
				Type:    "string",
				Pattern: `^[+-]?(0|([0-9]+(\.[0-9]+)?(ns|us|µs|μs|ms|s|m|h))+)$`,
			}
		}
		if t == reflect.TypeFor[zapcore.Level]() {
			return &jsonschema.Schema{
				Type: "string",
				Enum: []any{"debug", "info", "warn", "warning", "error", "panic", "fatal"},
			}
		}
		return nil
	}

	r.AdditionalFields = func(t reflect.Type) []reflect.StructField {
		if t.PkgPath() == "github.com/looplj/axonhub/internal/server" && t.Name() == "CORS" {
			return []reflect.StructField{
				{
					Name: "Debug",
					Type: reflect.TypeOf(false),
					Tag:  `json:"debug" yaml:"debug" conf:"debug"`,
				},
			}
		}
		return nil
	}

	s := r.Reflect(&conf.Config{})
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}
