package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
)

func init() {
	templates.CommonInitialisms["GT"] = true
	templates.CommonInitialisms["GTE"] = true

	templates.CommonInitialisms["LT"] = true
	templates.CommonInitialisms["LTE"] = true

	templates.CommonInitialisms["NEQ"] = true
	templates.CommonInitialisms["IDNEQ"] = true
}

func main() {
	log.SetOutput(io.Discard)
	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}

	err = api.Generate(cfg)

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}
}
