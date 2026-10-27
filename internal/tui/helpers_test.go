package tui_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/go-cmp/cmp"

	"github.com/elliottsam/kubectl-schematree/internal/schematest"
	"github.com/elliottsam/kubectl-schematree/internal/tui"
)

var update = flag.Bool("update", false, "rewrite golden files")

type harness struct {
	t *testing.T
	m *tui.Model
}

type option func(*tui.Config, *schematest.Discovery)

func withNoColor(c *tui.Config, _ *schematest.Discovery) { c.NoColor = true }

func withOpen(q string) option {
	return func(c *tui.Config, _ *schematest.Discovery) { c.Open = q }
}

func withDiscovery(fn func(*schematest.Discovery)) option {
	return func(_ *tui.Config, d *schematest.Discovery) { fn(d) }
}

// newHarness builds a model over the schematest documents at 100x30 and runs
// Init to completion.
func newHarness(t *testing.T, opts ...option) *harness {
	t.Helper()
	d := schematest.NewDiscovery()
	cfg := tui.Config{Context: "test"}
	for _, o := range opts {
		o(&cfg, d)
	}
	cfg.Source = schematest.Loader(t, d)
	m, err := tui.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, m: m}
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	h.run(m.Init())
	return h
}

// collect executes cmd, fanning out batches concurrently, and returns the
// resulting messages. Spinner ticks are dropped so loading cannot loop.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		results := make([][]tea.Msg, len(msg))
		var wg sync.WaitGroup
		for i, c := range msg {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i] = collect(c)
			}()
		}
		wg.Wait()
		var out []tea.Msg
		for _, r := range results {
			out = append(out, r...)
		}
		return out
	case spinner.TickMsg, nil:
		return nil
	default:
		return []tea.Msg{msg}
	}
}

func (h *harness) run(cmd tea.Cmd) {
	h.t.Helper()
	for _, msg := range collect(cmd) {
		h.send(msg)
	}
}

func (h *harness) send(msg tea.Msg) {
	h.t.Helper()
	_, cmd := h.m.Update(msg)
	h.run(cmd)
}

// key sends key presses named like the binding strings ("j", "down",
// "ctrl+d", "enter", "esc", "tab", "space", "G").
func (h *harness) key(names ...string) {
	h.t.Helper()
	for _, n := range names {
		h.send(keyMsg(n))
	}
}

// typeText sends each rune as a key press.
func (h *harness) typeText(s string) {
	h.t.Helper()
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func keyMsg(name string) tea.KeyPressMsg {
	switch name {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+d":
		return tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	case "ctrl+u":
		return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(name)
	if len(r) != 1 {
		panic("unknown key " + name)
	}
	return tea.KeyPressMsg{Code: r[0], Text: name}
}

// raw returns the rendered view including ANSI escapes.
func (h *harness) raw() string { return h.m.View().Content }

// plain returns the rendered view with ANSI escapes removed.
func (h *harness) plain() string { return ansi.Strip(h.raw()) }

func (h *harness) selected() string {
	n := h.m.Selected()
	if n == nil {
		return ""
	}
	return n.Label
}

// golden compares got with testdata/<name>.golden, rewriting it with -update.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}
	if diff := cmp.Diff(strings.TrimRight(string(want), "\n"), strings.TrimRight(got, "\n")); diff != "" {
		t.Fatalf("view differs from %s (-want +got):\n%s", path, diff)
	}
}
