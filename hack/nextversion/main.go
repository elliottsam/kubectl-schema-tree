// Command nextversion prints the next release tag, or nothing when the pull
// request opted out with "Release: none". It reads the existing tags
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
	fmt.Fprintf(os.Stderr, "bump: %s\n", bump)
	if bump == release.None {
		return nil
	}
	fmt.Println(release.Next(tags, bump))
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
