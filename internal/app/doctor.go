package app

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/spf13/cobra"
)

var doc = &cobra.Command{
	Use:   "doctor",
	Args:  cobra.NoArgs,
	Short: "Check Local env",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoc(cmd.OutOrStdout())
	},
}

func runDoc(out io.Writer) error {
	env, err := LoadEnvConfig()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "sage doctor")
	fmt.Fprintf(out, "[ok] os: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(out, "[ok] cwd path: %s\n", cwd)
	fmt.Fprintf(out, "[ok] Model: %s\n", env.Model)
	fmt.Fprintf(out, "[ok] API: %s\n", hideKey(env.OpenRouterKey))
	fmt.Fprintf(out, "[ok] URL: %s\n", env.BaseURL)
	return nil
}

func docCmd() *cobra.Command {
	return doc
}

func hideKey(key string) string {
	var count atomic.Int32
	count.Store(0)

	return strings.Map(func(r rune) rune {
		count.Add(1)
		if count.Load() > 5 {
			r = '*'
		}
		return r
	}, key)
}
