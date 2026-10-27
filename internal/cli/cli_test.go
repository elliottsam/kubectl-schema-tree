package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/elliottsam/kubectl-schematree/internal/cli"
	"github.com/elliottsam/kubectl-schematree/internal/schema"
	"github.com/elliottsam/kubectl-schematree/internal/schematest"
	"github.com/elliottsam/kubectl-schematree/internal/tui"
)

type env struct {
	out, errOut *bytes.Buffer
	cfg         cli.Config
	ran         *tui.Model
}

// newEnv returns a Config whose cluster is the schematest documents and whose
// program runner drives the model's Init to completion without a terminal.
func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	e.cfg = cli.Config{
		Streams: genericclioptions.IOStreams{In: strings.NewReader(""), Out: e.out, ErrOut: e.errOut},
		Version: "v1.2.3",
		Connect: func(*genericclioptions.ConfigFlags) (cli.Connection, error) {
			return cli.Connection{Source: schematest.Loader(t, schematest.NewDiscovery()), Context: "test-ctx"}, nil
		},
		RunProgram: func(_ context.Context, m tea.Model, _ genericclioptions.IOStreams) (tea.Model, error) {
			model := m.(*tui.Model)
			m, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			drive(m, model.Init())
			e.ran = model
			return m, nil
		},
		LookupEnv: func(string) (string, bool) { return "", false },
		CacheDir:  func() (string, error) { return t.TempDir(), nil },
	}
	return e
}

// drive executes cmd and feeds results to the model, ignoring spinner ticks.
func drive(m tea.Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			drive(m, c)
		}
	case spinner.TickMsg, nil:
	default:
		_, next := m.Update(msg)
		drive(m, next)
	}
}

func (e *env) execute(t *testing.T, args ...string) error {
	t.Helper()
	cmd := cli.NewCommand(e.cfg)
	cmd.SetArgs(args)
	return cmd.ExecuteContext(context.Background())
}

func TestNewCommandPanicsWithoutStreams(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing streams")
		}
	}()
	cli.NewCommand(cli.Config{})
}

func TestFlagsRegistered(t *testing.T) {
	t.Parallel()
	cmd := cli.NewCommand(newEnv(t).cfg)
	for _, name := range []string{"expand-depth", "no-color", "debug", "context", "kubeconfig", "namespace"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	if err := e.execute(t, "--version"); err != nil {
		t.Fatal(err)
	}
	if got := e.out.String(); got != "kubectl-schema_tree v1.2.3\n" {
		t.Fatalf("version output = %q", e.out.String())
	}
}

func TestRejectsTooManyArgs(t *testing.T) {
	t.Parallel()
	if err := newEnv(t).execute(t, "a", "b"); err == nil {
		t.Fatal("expected error for two positional arguments")
	}
}

func TestRejectsNegativeExpandDepth(t *testing.T) {
	t.Parallel()
	err := newEnv(t).execute(t, "--expand-depth", "-1")
	if err == nil || !strings.Contains(err.Error(), "expand-depth") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunStartsTUIWithContext(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	if err := e.execute(t); err != nil {
		t.Fatal(err)
	}
	view := ansi.Strip(e.ran.View().Content)
	if !strings.Contains(view, "ctx: test-ctx") || !strings.Contains(view, "apps/v1") {
		t.Fatalf("unexpected view:\n%s", view)
	}
}

func TestPositionalResourceIsOpened(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	if err := e.execute(t, "widget", "--expand-depth", "1"); err != nil {
		t.Fatal(err)
	}
	if got := e.ran.Selected().Label; got != "Widget" {
		t.Fatalf("selected = %q, want Widget", got)
	}
	if !strings.Contains(ansi.Strip(e.ran.View().Content), "spec") {
		t.Errorf("--expand-depth 1 did not expand children:\n%s", ansi.Strip(e.ran.View().Content))
	}
}

func TestNoColorFlagAndEnv(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want bool // true if the view must be free of colour escapes
	}{
		{"default-has-colour", nil, nil, false},
		{"flag", []string{"--no-color"}, nil, true},
		{"env", nil, map[string]string{"NO_COLOR": "1"}, true},
		{"empty-env-ignored", nil, map[string]string{"NO_COLOR": ""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.cfg.LookupEnv = func(k string) (string, bool) { v, ok := tt.env[k]; return v, ok }
			if err := e.execute(t, tt.args...); err != nil {
				t.Fatal(err)
			}
			raw := e.ran.View().Content
			hasColour := strings.Contains(raw, "38;2;") || strings.Contains(raw, "48;2;")
			if hasColour == tt.want {
				t.Fatalf("colour escapes present = %v, want %v", hasColour, !tt.want)
			}
		})
	}
}

func TestConnectErrorIsWrapped(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("no kubeconfig")
	e := newEnv(t)
	e.cfg.Connect = func(*genericclioptions.ConfigFlags) (cli.Connection, error) { return cli.Connection{}, sentinel }
	err := e.execute(t)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "connect to cluster") {
		t.Fatalf("err = %v", err)
	}
}

func TestProgramErrorIsWrapped(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("terminal gone")
	e := newEnv(t)
	e.cfg.RunProgram = func(context.Context, tea.Model, genericclioptions.IOStreams) (tea.Model, error) {
		return nil, sentinel
	}
	if err := e.execute(t); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v", err)
	}
}

func TestUnsupportedClusterGivesActionableError(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	d := schematest.NewDiscovery()
	d.PathsErr = schema.ErrOpenAPIV3Unsupported
	e.cfg.Connect = func(*genericclioptions.ConfigFlags) (cli.Connection, error) {
		return cli.Connection{Source: schematest.Loader(t, d), Context: "old"}, nil
	}
	err := e.execute(t)
	if !errors.Is(err, schema.ErrOpenAPIV3Unsupported) || !strings.Contains(err.Error(), "1.27") {
		t.Fatalf("err = %v", err)
	}
}

func TestDebugWritesLogFile(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := t.TempDir()
	e.cfg.CacheDir = func() (string, error) { return dir, nil }
	if err := e.execute(t, "--debug"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "kubectl-schema-tree", "debug.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "connected") || !strings.Contains(string(data), "test-ctx") {
		t.Errorf("log = %q", data)
	}
	if !strings.Contains(e.errOut.String(), path) {
		t.Errorf("stderr %q does not mention the log path", e.errOut.String())
	}
	// Nothing is written to stdout, which belongs to the TUI.
	if e.out.Len() != 0 {
		t.Errorf("stdout = %q", e.out.String())
	}
}

func TestDebugCacheDirError(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.cfg.CacheDir = func() (string, error) { return "", errors.New("no home") }
	if err := e.execute(t, "--debug"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNoDebugLeavesNoLog(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := t.TempDir()
	e.cfg.CacheDir = func() (string, error) { return dir, nil }
	if err := e.execute(t); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("unexpected files without --debug: %v", entries)
	}
}

const kubeconfig = `apiVersion: v1
kind: Config
current-context: ctx-a
clusters:
- name: c
  cluster: {server: "https://127.0.0.1:1"}
users:
- name: u
  user: {token: x}
contexts:
- name: ctx-a
  context: {cluster: c, user: u}
- name: ctx-b
  context: {cluster: c, user: u}
`

func TestConnectReadsContext(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(kubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, context, want string
	}{
		{"current-context", "", "ctx-a"},
		{"flag-overrides", "ctx-b", "ctx-b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			flags := genericclioptions.NewConfigFlags(true)
			flags.KubeConfig = &path
			cache := t.TempDir()
			flags.CacheDir = &cache
			if tt.context != "" {
				flags.Context = &tt.context
			}
			conn, err := cli.Connect(flags)
			if err != nil {
				t.Fatal(err)
			}
			if conn.Context != tt.want || conn.Source == nil {
				t.Fatalf("Connection = %+v, want context %q", conn, tt.want)
			}
		})
	}
}
