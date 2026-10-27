// Command kubectl-schema_tree is the kubectl plugin behind `kubectl schema-tree`.
package main

import (
	"os"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/elliottsam/kubectl-schema-tree/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cmd := cli.NewCommand(cli.Config{
		Streams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		Version: version,
	})
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
