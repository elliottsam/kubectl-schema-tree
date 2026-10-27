package tui_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/elliottsam/kubectl-schema-tree/internal/schematest"
	"github.com/elliottsam/kubectl-schema-tree/internal/tui"
)

func TestNewValidatesConfig(t *testing.T) {
	t.Parallel()
	if _, err := tui.New(tui.Config{}); err == nil {
		t.Fatal("expected error for missing Source")
	}
}

func TestInitListsGroups(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	view := h.plain()
	for _, want := range []string{"schema-tree", "ctx: test", "v1", "apps/v1"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if h.selected() != "v1" {
		t.Errorf("selected = %q, want v1", h.selected())
	}
}

func TestNavigation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		keys []string
		want string
	}{
		{"j-down", []string{"j"}, "apps/v1"},
		{"down-arrow", []string{"down"}, "apps/v1"},
		{"k-clamped", []string{"k"}, "v1"},
		{"G-bottom", []string{"G"}, "apps/v1"},
		{"g-top", []string{"G", "g"}, "v1"},
		{"expand-lazy-load", []string{"j", "l"}, "apps/v1"},
		{"step-into-after-load", []string{"j", "l", "l"}, "Gadget"},
		{"vim-descend", []string{"j", "l", "l", "j", "l", "l"}, "metadata"},
		{"arrow-descend", []string{"down", "right", "right"}, "Gadget"},
		{"h-jumps-to-parent", []string{"j", "l", "l", "h"}, "apps/v1"},
		{"left-arrow-parent", []string{"j", "l", "l", "left"}, "apps/v1"},
		{"enter-toggles", []string{"j", "enter", "enter", "enter"}, "apps/v1"},
		{"space-toggles", []string{"j", "space"}, "apps/v1"},
		{"ctrl-d-u", []string{"ctrl+d", "ctrl+u"}, "v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.key(tt.keys...)
			if got := h.selected(); got != tt.want {
				t.Fatalf("selected = %q, want %q\n%s", got, tt.want, h.plain())
			}
		})
	}
}

func TestExpandAndCollapseAll(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("j", "l", "l") // load apps/v1, move to Gadget
	h.key("g", "j", "E") // select apps/v1 and expand everything under it
	if !strings.Contains(h.plain(), "replicas") {
		t.Fatalf("E did not expand deeply:\n%s", h.plain())
	}
	h.key("C")
	if strings.Contains(h.plain(), "Gadget") {
		t.Fatalf("C did not collapse:\n%s", h.plain())
	}
}

// selectedLine returns the raw line whose tree pane (left of the first pane
// border pair) contains text, ignoring the detail pane.
func selectedLine(t *testing.T, raw, text string) string {
	t.Helper()
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.Split(line, "│")
		if len(parts) >= 3 && strings.Contains(ansi.Strip(parts[1]), text) {
			return line
		}
	}
	t.Fatalf("no tree row containing %q in:\n%s", text, ansi.Strip(raw))
	return ""
}

var bgRun = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// treeSection returns the content between the first two border runes of line.
func treeSection(t *testing.T, line string) string {
	t.Helper()
	parts := strings.Split(line, "│")
	if len(parts) < 3 {
		t.Fatalf("line has no pane border: %q", ansi.Strip(line))
	}
	return parts[1]
}

func TestSelectedRowHasFullWidthBackground(t *testing.T) {
	t.Parallel()
	const focusedBg = "48;2;31;74;122" // #1f4a7a, dark theme, tree focused
	h := newHarness(t)
	h.key("j", "l", "l") // select Gadget
	if h.selected() != "Gadget" {
		t.Fatalf("selected = %q", h.selected())
	}

	line := selectedLine(t, h.raw(), "Gadget")
	section := treeSection(t, line)
	if got := ansi.StringWidth(section); got != 48 {
		t.Fatalf("tree pane width = %d, want 48", got)
	}
	// Every styled run on the selected row must carry the background, which
	// proves the bar spans labels, type text and trailing padding.
	runs := strings.Split(section, "\x1b[m")
	checked := 0
	for _, run := range runs {
		if ansi.Strip(run) == "" {
			continue
		}
		checked++
		if !strings.Contains(run, focusedBg) {
			t.Errorf("run %q has no selection background", run)
		}
	}
	if checked < 3 {
		t.Fatalf("only %d runs on selected row; expected label, type and padding", checked)
	}
	// Exactly one row carries the bar.
	if n := strings.Count(h.raw(), focusedBg+"m"); n == 0 {
		t.Fatal("no background in view")
	}
	for _, l := range strings.Split(h.raw(), "\n") {
		parts := strings.Split(l, "│")
		if len(parts) >= 3 && strings.Contains(parts[1], focusedBg) && !strings.Contains(ansi.Strip(parts[1]), "Gadget") {
			t.Errorf("background on a non-selected row: %q", ansi.Strip(l))
		}
	}
}

func TestSelectionBackgroundFollowsCursor(t *testing.T) {
	t.Parallel()
	const bg = "48;2;31;74;122"
	h := newHarness(t)
	h.key("j", "l", "l", "j") // Gadget -> Widget
	if h.selected() != "Widget" {
		t.Fatalf("selected = %q", h.selected())
	}
	if !strings.Contains(selectedLine(t, h.raw(), "Widget"), bg) {
		t.Error("Widget row lacks the selection background")
	}
	if strings.Contains(selectedLine(t, h.raw(), "Gadget"), bg) {
		t.Error("Gadget row still highlighted after the cursor moved")
	}
}

func TestSelectionBackgroundDimsWhenDetailFocused(t *testing.T) {
	t.Parallel()
	const focused, dimmed = "48;2;31;74;122", "48;2;51;54;60"
	h := newHarness(t)
	h.key("j", "l", "l")
	h.key("tab")
	if h.m.Focus() != tui.FocusDetail {
		t.Fatalf("focus = %v", h.m.Focus())
	}
	line := selectedLine(t, h.raw(), "Gadget")
	if strings.Contains(line, focused) || !strings.Contains(line, dimmed) {
		t.Errorf("selected row not dimmed when detail focused: %q", line)
	}
	h.key("tab")
	if !strings.Contains(selectedLine(t, h.raw(), "Gadget"), focused) {
		t.Error("highlight did not return to the focused colour")
	}
}

func TestSelectionUsesReverseVideoWithoutColor(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withNoColor)
	h.key("j", "l", "l")
	raw := h.raw()
	line := selectedLine(t, raw, "Gadget")
	if !strings.Contains(line, "\x1b[7m") {
		t.Errorf("selected row lacks reverse video: %q", line)
	}
	if strings.Contains(raw, "48;2;") || strings.Contains(raw, "38;2;") {
		t.Error("colour escapes present with NoColor")
	}
	// Other rows must not be reversed.
	if strings.Contains(selectedLine(t, raw, "Widget"), "\x1b[7m") {
		t.Error("non-selected row is reversed")
	}
}

func TestLightBackgroundUsesLightSelection(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	dark := selectedLine(t, h.raw(), "v1")
	h.send(tea.BackgroundColorMsg{Color: lightColor{}})
	light := selectedLine(t, h.raw(), "v1")
	if dark == light {
		t.Fatal("light terminal rendered identically to dark")
	}
	if !strings.Contains(light, "48;2;191;219;254") { // #bfdbfe
		t.Errorf("light selection background missing: %q", light)
	}
}

type lightColor struct{}

func (lightColor) RGBA() (r, g, b, a uint32) { return 0xffff, 0xffff, 0xffff, 0xffff }

func TestDetailPane(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("j", "l", "l") // load apps/v1, select Gadget
	h.key("l", "l")      // expand Gadget, step into replicas
	if h.selected() != "replicas" {
		t.Fatalf("selected = %q\n%s", h.selected(), h.plain())
	}
	view := h.plain()
	for _, want := range []string{"Gadget.replicas", "apps/v1/Gadget", "integer (int32)", "optional", "default: 1", "Desired count."} {
		if !strings.Contains(view, want) {
			t.Errorf("detail missing %q:\n%s", want, view)
		}
	}
}

func TestDetailForResourceRootIsNotOptional(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withOpen("gadget"))
	view := h.plain()
	if !strings.Contains(view, "Gadget · resource") || strings.Contains(view, "optional") {
		t.Errorf("resource root detail wrong:\n%s", view)
	}
}

func TestDetailShowsRequiredEnumAndGroup(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("j")
	if !strings.Contains(h.plain(), "Not loaded") {
		t.Errorf("unloaded group detail missing:\n%s", h.plain())
	}
	h = newHarness(t, withOpen("widget"))
	h.key("l", "j", "l", "l") // metadata, spec, expand spec, into color
	if h.selected() != "color" {
		t.Fatalf("selected = %q\n%s", h.selected(), h.plain())
	}
	if !strings.Contains(h.plain(), "enum: red, green") {
		t.Errorf("enum missing:\n%s", h.plain())
	}
	h.key("k") // replicas
	if !strings.Contains(h.plain(), "required") {
		t.Errorf("required missing:\n%s", h.plain())
	}
}

func TestDetailScrollsIndependently(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("j", "l", "l")
	before := h.selected()
	h.key("tab", "j", "j", "ctrl+d")
	if h.selected() != before {
		t.Fatalf("tree cursor moved while detail focused: %q", h.selected())
	}
	h.key("tab")
	if h.m.Focus() != tui.FocusTree {
		t.Fatalf("focus = %v", h.m.Focus())
	}
}

func TestSearchFiltersAndHighlights(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("/")
	if h.m.Focus() != tui.FocusSearch {
		t.Fatalf("focus = %v", h.m.Focus())
	}
	h.typeText("color")

	view := h.plain()
	if !strings.Contains(view, "color") || strings.Contains(view, "Gadget") {
		t.Errorf("filter wrong:\n%s", view)
	}
	if !strings.Contains(view, "matches") {
		t.Errorf("footer lacks match count:\n%s", view)
	}
	if h.selected() != "color" || !h.m.Filtering() {
		t.Fatalf("selected=%q filtering=%v", h.selected(), h.m.Filtering())
	}
	// Matched characters are drawn bold and underlined (SGR 1 and 4).
	line := selectedLine(t, h.raw(), "color")
	if !strings.Contains(line, ";4") && !strings.Contains(line, "[4") {
		t.Errorf("matched characters not underlined: %q", line)
	}

	h.key("enter")
	if h.m.Focus() != tui.FocusTree || !h.m.Filtering() {
		t.Fatalf("enter should keep the filter: focus=%v filtering=%v", h.m.Focus(), h.m.Filtering())
	}
	h.key("esc")
	if h.m.Filtering() {
		t.Fatal("esc did not clear the filter")
	}
	if !strings.Contains(h.plain(), "apps/v1") {
		t.Errorf("tree not restored:\n%s", h.plain())
	}
}

func TestSearchNoMatches(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("/")
	h.typeText("zzzzqq")
	if !strings.Contains(h.plain(), "no matches") {
		t.Errorf("missing no-matches message:\n%s", h.plain())
	}
	if h.selected() != "" {
		t.Errorf("selected = %q with no matches", h.selected())
	}
	h.key("esc")
	if h.m.Focus() != tui.FocusTree || h.selected() == "" {
		t.Fatalf("esc did not restore: focus=%v selected=%q", h.m.Focus(), h.selected())
	}
}

func TestSearchCyclesMatches(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("/")
	h.typeText("replicas")
	h.key("enter")
	first := h.m.Selected()
	h.key("n")
	if h.m.Selected() == first {
		t.Fatal("n did not move to another match")
	}
	h.key("N")
	if h.m.Selected() != first {
		t.Fatal("N did not return to the first match")
	}
}

func TestSearchLoadsAllGroups(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("/")
	h.typeText("nodeName")
	if !strings.Contains(h.plain(), "nodeName") {
		t.Errorf("search did not reach an unexpanded group:\n%s", h.plain())
	}
}

func TestForbiddenGroupIsReported(t *testing.T) {
	t.Parallel()
	gr := k8sschema.GroupResource{Group: "apps", Resource: "x"}
	h := newHarness(t, withDiscovery(func(d *schematest.Discovery) {
		d.DocErrFor = map[string]error{"apis/apps/v1": apierrors.NewForbidden(gr, "x", errors.New("denied"))}
	}))
	h.key("j", "l")
	view := h.plain()
	if !strings.Contains(view, "forbidden") {
		t.Errorf("forbidden not shown:\n%s", view)
	}
	h.key("l") // a failed group has no children to step into
	if h.selected() != "apps/v1" {
		t.Errorf("selected = %q", h.selected())
	}
}

func TestFatalErrorWhenOpenAPIUnsupported(t *testing.T) {
	t.Parallel()
	gr := k8sschema.GroupResource{Group: "x", Resource: "y"}
	h := newHarness(t, withDiscovery(func(d *schematest.Discovery) {
		d.PathsErr = apierrors.NewNotFound(gr, "openapi")
	}))
	if h.m.Err() == nil {
		t.Fatal("expected fatal error")
	}
	if !strings.Contains(h.plain(), "OpenAPI v3") || !strings.Contains(h.plain(), "press q") {
		t.Errorf("fatal view lacks guidance:\n%s", h.plain())
	}
	_, cmd := h.m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("q did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q returned %T", cmd())
	}
}

func TestQuitKeys(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"q", "ctrl+c"} {
		h := newHarness(t)
		_, cmd := h.m.Update(keyMsg(k))
		if cmd == nil {
			t.Fatalf("%s returned no command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s returned %T", k, cmd())
		}
	}
	// q is text while searching.
	h := newHarness(t)
	h.key("/")
	if _, cmd := h.m.Update(keyMsg("q")); cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("q quit while typing a search")
		}
	}
}

func TestHelpOverlay(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("?")
	if !strings.Contains(h.plain(), "half page down") {
		t.Errorf("full help not shown:\n%s", h.plain())
	}
	h.key("x")
	if strings.Contains(h.plain(), "half page down") {
		t.Error("help not dismissed")
	}
}

func TestOpen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		query string
		want  string
	}{
		{"widget", "Widget"},
		{"Gadget", "Gadget"},
		{"apps/v1/gadget", "Gadget"},
		{"pods", "Pod"},
		{"v1/Pod", "Pod"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withOpen(tt.query))
			if h.selected() != tt.want {
				t.Fatalf("selected = %q, want %q\n%s", h.selected(), tt.want, h.plain())
			}
			if !h.m.Selected().Expanded() {
				t.Error("opened resource is not expanded")
			}
		})
	}
}

func TestOpenNotFound(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withOpen("nope"))
	if !strings.Contains(h.plain(), `resource "nope" not found`) {
		t.Errorf("missing not-found notice:\n%s", h.plain())
	}
}

// TestLayoutFitsTerminal checks every size renders exactly width x height.
func TestLayoutFitsTerminal(t *testing.T) {
	t.Parallel()
	sizes := [][2]int{{120, 40}, {100, 30}, {80, 24}, {79, 24}, {60, 20}, {40, 10}, {30, 6}}
	for _, sz := range sizes {
		for _, focus := range []string{"tree", "detail"} {
			h := newHarness(t)
			h.key("j", "l", "l", "l")
			if focus == "detail" {
				h.key("tab")
			}
			h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			lines := strings.Split(h.plain(), "\n")
			if len(lines) != sz[1] {
				t.Errorf("%dx%d %s: %d lines, want %d", sz[0], sz[1], focus, len(lines), sz[1])
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w != sz[0] {
					t.Errorf("%dx%d %s: line %d width %d, want %d: %q", sz[0], sz[1], focus, i, w, sz[0], l)
					break
				}
			}
		}
	}
}

func TestNarrowShowsOnePane(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("j", "l", "l")
	h.send(tea.WindowSizeMsg{Width: 60, Height: 20})
	if strings.Contains(h.plain(), "optional") {
		t.Errorf("detail pane visible in narrow mode:\n%s", h.plain())
	}
	h.key("tab")
	if !strings.Contains(h.plain(), "Gadget is another thing.") {
		t.Errorf("tab did not show the detail pane:\n%s", h.plain())
	}
}

func TestKeyMapIsComplete(t *testing.T) {
	t.Parallel()
	for _, b := range tui.DefaultKeyMap().All() {
		if len(b.Keys()) == 0 || b.Help().Key == "" || b.Help().Desc == "" {
			t.Errorf("incomplete binding: keys=%v help=%+v", b.Keys(), b.Help())
		}
	}
}

func TestGoldenExpandedTree(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withOpen("widget"))
	h.key("l", "j", "l", "l") // metadata, spec, expand spec, into color
	golden(t, "expanded_tree", h.plain())
}

func TestGoldenSearch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.key("/")
	h.typeText("replicas")
	golden(t, "search_replicas", h.plain())
}

func TestGoldenNoColorHasNoEscapes(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withNoColor)
	h.key("j", "l", "l")
	raw := h.raw()
	// Only attribute escapes (bold, reverse, faint, underline) may appear.
	for _, esc := range bgRun.FindAllString(raw, -1) {
		if strings.Contains(esc, "38;") || strings.Contains(esc, "48;") {
			t.Fatalf("colour escape %q with NoColor", esc)
		}
	}
}
