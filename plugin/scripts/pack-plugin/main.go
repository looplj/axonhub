package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/looplj/axonhub/internal/server/plugin"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("pack-plugin", flag.ContinueOnError)
	flags.SetOutput(stdout)
	var output string
	flags.StringVar(&output, "o", "", "output path (default: <parent>/<directory-name>.axplugin)")
	flags.StringVar(&output, "output", "", "output path")
	flags.Usage = func() {
		fmt.Fprintln(stdout, "Usage: pack-plugin <plugin-directory> [-o output.axplugin]")
		flags.PrintDefaults()
	}
	// Accept the directory before flags, as in the previous Python command.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("exactly one plugin directory is required")
	}
	root, err := filepath.Abs(flags.Arg(0))
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if output == "" {
		output = filepath.Join(filepath.Dir(root), filepath.Base(root)+".axplugin")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return err
	}
	output = filepath.Join(parent, filepath.Base(output))
	rel, err := filepath.Rel(root, output)
	if err != nil {
		return err
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("output must be outside the plugin directory")
	}
	data, err := plugin.PackDirectory(root)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(parent, ".pack-plugin-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), output); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s (%d bytes)\n", output, len(data))
	return err
}
