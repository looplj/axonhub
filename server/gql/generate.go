package main

import (
	"fmt"
	"os"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin/modelgen"
)

var (
	omitableFields = []OmitableField{
		{
			Model: "CreateChannelInput",
			Field: "Settings",
		},
		{
			Model: "UpdateChannelInput",
			Field: "Settings",
		},
	}
)

func IsOmitableField(model, field string) bool {
	for _, omit := range omitableFields {
		if omit.Model == model && omit.Field == field {
			return true
		}
	}

	return false
}

type OmitableField struct {
	Model string
	Field string
}

// Defining mutation function
func mutateHook(b *modelgen.ModelBuild) *modelgen.ModelBuild {
	for _, model := range b.Models {
		for _, field := range model.Fields {
			if IsOmitableField(model.Name, field.Name) {
				field.Omittable = true
			}
		}
	}

	return b
}

func main() {
	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}

	// Attaching the mutation function onto modelgen plugin
	p := modelgen.Plugin{
		MutateHook: mutateHook,
	}

	err = api.Generate(cfg, api.ReplacePlugin(&p))

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}
}
