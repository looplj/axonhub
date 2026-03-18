package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/looplj/axonhub/conf"
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

	s := r.Reflect(&conf.Config{})
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}
