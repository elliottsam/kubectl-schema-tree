//go:build ignore

// Command fixtures captures trimmed OpenAPI v3 documents from a live cluster
// for use as test fixtures. Run it through `make fixtures`.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/pflag"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// capture lists, per discovery path, the schema names to keep as roots.
var capture = map[string][]string{
	"api/v1":                       {"io.k8s.api.core.v1.ConfigMap", "io.k8s.api.core.v1.Service"},
	"apis/apps/v1":                 {"io.k8s.api.apps.v1.Deployment"},
	"apis/apiextensions.k8s.io/v1": {"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceDefinition"},
}

func main() {
	out := pflag.String("out", "internal/schema/testdata", "output directory")
	flags := genericclioptions.NewConfigFlags(true)
	flags.AddFlags(pflag.CommandLine)
	pflag.Parse()

	if err := run(flags, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(flags *genericclioptions.ConfigFlags, out string) error {
	dc, err := flags.ToDiscoveryClient()
	if err != nil {
		return err
	}
	paths, err := dc.OpenAPIV3().Paths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for path, roots := range capture {
		gv, ok := paths[path]
		if !ok {
			return fmt.Errorf("path %q not served", path)
		}
		data, err := gv.Schema("application/json")
		if err != nil {
			return err
		}
		trimmed, err := trim(data, roots)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		name := strings.ReplaceAll(path, "/", "_") + ".json"
		if err := os.WriteFile(filepath.Join(out, name), trimmed, 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s (%d bytes)\n", name, len(trimmed))
	}
	return nil
}

// trim keeps only the schemas reachable from roots and drops the paths.
func trim(data []byte, roots []string) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	comps, _ := doc["components"].(map[string]any)
	schemas, _ := comps["schemas"].(map[string]any)

	keep := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		if keep[name] {
			return
		}
		s, ok := schemas[name]
		if !ok {
			return
		}
		keep[name] = true
		walkRefs(s, visit)
	}
	for _, r := range roots {
		if _, ok := schemas[r]; !ok {
			return nil, fmt.Errorf("root schema %q not found", r)
		}
		visit(r)
	}
	kept := map[string]any{}
	for name := range keep {
		kept[name] = schemas[name]
	}
	out := map[string]any{
		"openapi":    doc["openapi"],
		"info":       doc["info"],
		"components": map[string]any{"schemas": kept},
	}
	return json.MarshalIndent(out, "", " ")
}

func walkRefs(v any, visit func(string)) {
	switch t := v.(type) {
	case map[string]any:
		if ref, ok := t["$ref"].(string); ok {
			visit(strings.TrimPrefix(ref, "#/components/schemas/"))
		}
		for _, c := range t {
			walkRefs(c, visit)
		}
	case []any:
		for _, c := range t {
			walkRefs(c, visit)
		}
	}
}
