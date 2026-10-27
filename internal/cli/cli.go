// Package cli wires command-line flags, the Kubernetes connection and the TUI
// together. It is the only package that logs.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/elliottsam/kubectl-schematree/internal/schema"
	"github.com/elliottsam/kubectl-schematree/internal/tui"
)

// Connection is a schema source plus the Kubernetes context it talks to.
type Connection struct {
	Source  tui.Source
	Context string
}

// Config configures NewCommand.
type Config struct {
	// Streams are the standard streams. Required.
	Streams genericclioptions.IOStreams
	// Version is shown by --version.
	Version string
	// Connect builds the schema source from kubectl connection flags. It
	// defaults to connecting to the cluster selected by the kubeconfig.
	Connect func(flags *genericclioptions.ConfigFlags) (Connection, error)
	// RunProgram runs the TUI model to completion. It defaults to a Bubble
	// Tea program on the alternate screen.
	RunProgram func(ctx context.Context, m tea.Model, streams genericclioptions.IOStreams) (tea.Model, error)
	// LookupEnv reads environment variables. It defaults to os.LookupEnv.
	LookupEnv func(string) (string, bool)
	// CacheDir is where --debug writes its log. It defaults to os.UserCacheDir.
	CacheDir func() (string, error)
}

func (c *Config) applyDefaults() {
	if c.Connect == nil {
		c.Connect = connect
	}
	if c.RunProgram == nil {
		c.RunProgram = runProgram
	}
	if c.LookupEnv == nil {
		c.LookupEnv = os.LookupEnv
	}
	if c.CacheDir == nil {
		c.CacheDir = os.UserCacheDir
	}
}

// Validate reports whether the Config can be used.
func (c Config) Validate() error {
	if c.Streams.Out == nil || c.Streams.ErrOut == nil {
		return errors.New("cli: Config.Streams must have Out and ErrOut")
	}
	return nil
}

type options struct {
	cfg         Config
	flags       *genericclioptions.ConfigFlags
	expandDepth int
	noColor     bool
	debug       bool
}

// NewCommand returns the root `kubectl schema-tree` command. It panics if cfg
// is invalid, because that is a programming error in main.
func NewCommand(cfg Config) *cobra.Command {
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	cfg.applyDefaults()
	o := &options{cfg: cfg, flags: genericclioptions.NewConfigFlags(true)}

	cmd := &cobra.Command{
		Use:   "kubectl schema-tree [RESOURCE]",
		Short: "Browse Kubernetes API schemas in an interactive tree",
		Long: `Browse the OpenAPI v3 schemas served by a cluster in an interactive tree.

RESOURCE optionally selects a resource on start-up, either a kind such as
"deployment" or a full "apps/v1/Deployment". The cluster must serve
/openapi/v3 (Kubernetes 1.27 or newer).`,
		Example: `  kubectl schema-tree
  kubectl schema-tree deployment
  kubectl schema-tree apps/v1/Deployment --expand-depth 2
  kubectl schema-tree --context staging crontab`,
		Args:          cobra.MaximumNArgs(1),
		Version:       cfg.Version,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			open := ""
			if len(args) == 1 {
				open = args[0]
			}
			return o.run(cmd.Context(), open)
		},
	}
	cmd.SetVersionTemplate("kubectl-schema_tree {{.Version}}\n")
	cmd.SetOut(cfg.Streams.Out)
	cmd.SetErr(cfg.Streams.ErrOut)
	cmd.SetIn(cfg.Streams.In)

	o.flags.AddFlags(cmd.Flags())
	cmd.Flags().IntVar(&o.expandDepth, "expand-depth", 0, "levels to pre-expand below the selected RESOURCE")
	cmd.Flags().BoolVar(&o.noColor, "no-color", false, "disable colours (also honours NO_COLOR)")
	cmd.Flags().BoolVar(&o.debug, "debug", false, "write a debug log to the user cache directory")
	return cmd
}

func (o *options) run(ctx context.Context, open string) error {
	if o.expandDepth < 0 {
		return fmt.Errorf("--expand-depth must not be negative, got %d", o.expandDepth)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	log, closeLog, err := o.logger()
	if err != nil {
		return err
	}
	defer closeLog()

	conn, err := o.cfg.Connect(o.flags)
	if err != nil {
		return fmt.Errorf("connect to cluster: %w", err)
	}
	log.Info("connected", "context", conn.Context, "open", open)

	model, err := tui.New(tui.Config{
		Source:      conn.Source,
		Context:     conn.Context,
		ExpandDepth: o.expandDepth,
		NoColor:     o.noColor || o.envNoColor(),
		Open:        open,
		Ctx:         ctx,
	})
	if err != nil {
		return err
	}
	final, err := o.cfg.RunProgram(ctx, model, o.cfg.Streams)
	if err != nil {
		log.Error("program failed", "error", err)
		return fmt.Errorf("run TUI: %w", err)
	}
	if m, ok := final.(*tui.Model); ok && m.Err() != nil {
		log.Error("schema load failed", "error", m.Err())
		return explain(m.Err())
	}
	return nil
}

// explain turns sentinel errors into actionable messages.
func explain(err error) error {
	switch {
	case errors.Is(err, schema.ErrOpenAPIV3Unsupported):
		return fmt.Errorf("%w; upgrade the cluster to Kubernetes 1.27 or newer", err)
	case errors.Is(err, schema.ErrForbidden):
		return fmt.Errorf("%w; check your RBAC permissions for the /openapi/v3 endpoint", err)
	default:
		return err
	}
}

// envNoColor reports whether NO_COLOR requests plain output.
func (o *options) envNoColor() bool {
	v, ok := o.cfg.LookupEnv("NO_COLOR")
	return ok && v != ""
}

// logger returns a logger that discards output unless --debug is set, in
// which case it writes to a file (stdout and stderr belong to the TUI).
func (o *options) logger() (*slog.Logger, func(), error) {
	if !o.debug {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() {}, nil
	}
	dir, err := o.cfg.CacheDir()
	if err != nil {
		return nil, nil, fmt.Errorf("find cache directory for debug log: %w", err)
	}
	dir = filepath.Join(dir, "kubectl-schema-tree")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create debug log directory: %w", err)
	}
	path := filepath.Join(dir, "debug.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open debug log: %w", err)
	}
	_, _ = fmt.Fprintf(o.cfg.Streams.ErrOut, "debug log: %s\n", path)
	log := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return log, func() { _ = f.Close() }, nil
}

// connect builds a schema Loader for the cluster selected by flags.
func connect(flags *genericclioptions.ConfigFlags) (Connection, error) {
	dc, err := flags.ToDiscoveryClient()
	if err != nil {
		return Connection{}, err
	}
	loader, err := schema.NewLoader(schema.Config{Discovery: dc})
	if err != nil {
		return Connection{}, err
	}
	return Connection{Source: loader, Context: contextName(flags)}, nil
}

func contextName(flags *genericclioptions.ConfigFlags) string {
	if flags.Context != nil && *flags.Context != "" {
		return *flags.Context
	}
	raw, err := flags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return ""
	}
	return raw.CurrentContext
}

func runProgram(ctx context.Context, m tea.Model, streams genericclioptions.IOStreams) (tea.Model, error) {
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(streams.In), tea.WithOutput(streams.Out))
	return p.Run()
}
