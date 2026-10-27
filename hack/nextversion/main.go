// Command nextversion prints the next release tag. It reads the existing tags
// from the current git repository and the pull request description from the
// file named by -body-file ("-" for stdin, omitted for none).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/elliottsam/kubectl-schema-tree/internal/release"
)

func main() {
	bodyFile := flag.String("body-file", "", `file holding the pull request description ("-" for stdin)`)
	flag.Parse()

	if err := run(*bodyFile); err != nil {
		fmt.Fprintln(os.Stderr, "nextversion:", err)
		os.Exit(1)
	}
}

func run(bodyFile string) error {
	body, err := readBody(bodyFile)
	if err != nil {
		return err
	}
	out, err := exec.Command("git", "tag", "--list").Output()
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	tags := strings.Fields(string(out))

	bump := release.BumpFromDescription(body)
	next := release.Next(tags, bump)
	fmt.Fprintf(os.Stderr, "bump: %s\n", bump)
	fmt.Println(next)
	return nil
}

func readBody(path string) (string, error) {
	switch path {
	case "":
		return "", nil
	case "-":
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	default:
		b, err := os.ReadFile(path)
		return string(b), err
	}
}
