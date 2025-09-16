package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ipfs/boxo/mfs"
	"github.com/ipfs/go-cid"
	"github.com/spf13/cobra"

	unixfs "github.com/gosuda/boxo-starter-kit/05-unixfs-car/pkg"
	mymfs "github.com/gosuda/boxo-starter-kit/06-mfs/pkg"
)

type State struct {
	Root string `json:"root"` // CID string (may be empty)
}

func repoDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mfs-mini")
}
func statePath() string { return filepath.Join(repoDir(), "state.json") }

func loadState() (State, error) {
	_ = os.MkdirAll(repoDir(), 0o755)
	b, err := os.ReadFile(statePath())
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	return s, json.Unmarshal(b, &s)
}
func saveState(s State) error {
	_ = os.MkdirAll(repoDir(), 0o755)
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(statePath(), b, 0o644)
}

/********** helpers **********/
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type App struct {
	ctx   context.Context
	state State
	ufs   *unixfs.UnixFsWrapper
	mfsw  *mymfs.MFSWrapper
}

func NewApp() (*App, error) {
	app := &App{}
	err := app.setup()
	return app, err
}

func (a *App) setup() error {
	a.ctx = context.Background()

	state, err := loadState()
	if err != nil {
		return err
	}
	a.state = state

	ufs, err := unixfs.New(0, nil)
	if err != nil {
		return err
	}
	a.ufs = ufs

	var rootCID cid.Cid
	if a.state.Root != "" {
		rootCID, err = cid.Parse(a.state.Root)
		if err != nil {
			return err
		}
	}

	mfsw, err := mymfs.New(a.ctx, a.ufs, rootCID)
	if err != nil {
		return err
	}
	a.mfsw = mfsw

	return nil
}

func (a *App) commitAndPrint() error {
	c, err := a.mfsw.SnapshotCID(a.ctx)
	if err != nil {
		return err
	}
	a.state.Root = c.String()
	if err := saveState(a.state); err != nil {
		return err
	}
	fmt.Println(c)
	return nil
}

type CommandOptions struct {
	writeAppend bool
}

type CommandHandler func(*App, *CommandOptions, []string) error

func wrapCommand(handler CommandHandler) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		app, err := NewApp()
		must(err)

		opts := &CommandOptions{
			writeAppend: writeAppend,
		}

		err = handler(app, opts, args)
		must(err)
	}
}

var writeAppend bool

var rootCmd = &cobra.Command{
	Use:   "mfs-mini",
	Short: "Tiny MFS CLI",
	Long:  "mfs-mini - tiny MFS CLI",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new empty root and save state",
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		return app.commitAndPrint()
	}),
}

var lsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List directory",
	Args:  cobra.MaximumNArgs(1),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		p := "/"
		if len(args) == 1 {
			p = mymfs.NormPath(args[0])
		}
		fsn, err := mfs.Lookup(app.mfsw.Root(), mymfs.NormPath(p))
		if err != nil {
			return err
		}

		switch n := fsn.(type) {
		case *mfs.Directory:
			entries, err := n.List(app.ctx)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if e.Type == int(mfs.TDir) {
					fmt.Println(e.Name + "/")
				} else {
					fmt.Println(e.Name)
				}
			}
		default:
			fmt.Println(filepath.Base(p))
		}
		return nil
	}),
}

var catCmd = &cobra.Command{
	Use:   "cat <path>",
	Short: "Print file contents",
	Args:  cobra.ExactArgs(1),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		data, err := app.mfsw.ReadBytes(app.ctx, mymfs.NormPath(args[0]))
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	}),
}

var putCmd = &cobra.Command{
	Use:   "put <src-file> <dst-path-in-mfs>",
	Short: "Put local file into MFS",
	Args:  cobra.ExactArgs(2),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		src := args[0]
		dst := mymfs.NormPath(args[1])
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := app.mfsw.WriteBytes(app.ctx, dst, b, true); err != nil {
			return err
		}
		return app.commitAndPrint()
	}),
}

var writeCmd = &cobra.Command{
	Use:   "write <dst-path-in-mfs> <string>...",
	Short: "Write a string to a file (use --append to append)",
	Args:  cobra.MinimumNArgs(2),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		dst := mymfs.NormPath(args[0])
		payload := []byte(strings.Join(args[1:], " "))

		if opts.writeAppend {
			old, err := app.mfsw.ReadBytes(app.ctx, dst)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			payload = append(old, payload...)
		}

		if err := app.mfsw.WriteBytes(app.ctx, dst, payload, !opts.writeAppend); err != nil {
			return err
		}
		return app.commitAndPrint()
	}),
}

var mkdirCmd = &cobra.Command{
	Use:   "mkdir <path>",
	Short: "Create directory (parents=true)",
	Args:  cobra.ExactArgs(1),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		if err := mfs.Mkdir(app.mfsw.Root(), mymfs.NormPath(args[0]), mfs.MkdirOpts{Mkparents: true}); err != nil {
			return err
		}
		return app.commitAndPrint()
	}),
}

var mvCmd = &cobra.Command{
	Use:   "mv <src> <dst>",
	Short: "Move/rename a path",
	Args:  cobra.ExactArgs(2),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		if err := app.mfsw.Move(app.ctx, mymfs.NormPath(args[0]), mymfs.NormPath(args[1])); err != nil {
			return err
		}
		return app.commitAndPrint()
	}),
}

var rmCmd = &cobra.Command{
	Use:   "rm <path>",
	Short: "Remove a file or directory",
	Args:  cobra.ExactArgs(1),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		if err := app.mfsw.Remove(app.ctx, mymfs.NormPath(args[0])); err != nil {
			return err
		}
		return app.commitAndPrint()
	}),
}

var chmodCmd = &cobra.Command{
	Use:   "chmod <octal> <path>",
	Short: "Change file mode (e.g. 0644)",
	Args:  cobra.ExactArgs(2),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		var mode uint32
		if _, err := fmt.Sscanf(args[0], "%o", &mode); err != nil {
			return err
		}
		if err := app.mfsw.Chmod(app.ctx, mymfs.NormPath(args[1]), mode); err != nil {
			return err
		}
		return app.commitAndPrint()
	}),
}

var touchCmd = &cobra.Command{
	Use:   "touch <path>",
	Short: "Update mtime or create file",
	Args:  cobra.ExactArgs(1),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		if err := app.mfsw.Touch(app.ctx, mymfs.NormPath(args[0]), time.Now()); err != nil {
			return err
		}
		return app.commitAndPrint()
	}),
}

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Take snapshot of root and print CID",
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		return app.commitAndPrint()
	}),
}

var exportCmd = &cobra.Command{
	Use:   "export <car-file>",
	Short: "Export snapshot to CAR",
	Args:  cobra.ExactArgs(1),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		f, err := os.Create(args[0])
		if err != nil {
			return err
		}
		defer f.Close()

		type ws interface {
			io.Writer
			io.Seeker
		}
		w, ok := any(f).(ws)
		if !ok {
			return fmt.Errorf("file not seekable")
		}

		return app.mfsw.ExportCAR(app.ctx, w)
	}),
}

var importCmd = &cobra.Command{
	Use:   "import <car-file>",
	Short: "Import a CAR and set root to its snapshot",
	Args:  cobra.ExactArgs(1),
	Run: wrapCommand(func(app *App, opts *CommandOptions, args []string) error {
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()

		choose := func(roots []cid.Cid) cid.Cid {
			return roots[0]
		}

		if _, err := app.mfsw.ImportCAR(app.ctx, f, choose); err != nil {
			return err
		}
		return app.commitAndPrint()
	}),
}

func init() {
	writeCmd.Flags().BoolVar(&writeAppend, "append", false, "append instead of truncate")

	rootCmd.AddCommand(
		initCmd,
		lsCmd,
		catCmd,
		putCmd,
		writeCmd,
		mkdirCmd,
		mvCmd,
		rmCmd,
		chmodCmd,
		touchCmd,
		snapshotCmd,
		exportCmd,
		importCmd,
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
