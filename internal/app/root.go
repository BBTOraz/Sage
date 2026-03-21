package app

import (
	"bilge-lib/internal/approval"
	"context"
	"errors"

	"github.com/spf13/cobra"
)

type Config struct {
	Mode approval.Mode
}

func rootCmd(ctx context.Context) *cobra.Command {
	var guard, full bool
	rcmd := &cobra.Command{
		Use:   "sage",
		Short: "Chat CLI agent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := resolveConfig(guard, full)
			if err != nil {
				return err
			}
			return Run(ctx, cfg)
		},
	}
	rcmd.AddCommand(docCmd(), versionCmd())
	rcmd.Flags().BoolVarP(&guard, "guard", "g", false, "require approval before execute command")
	rcmd.Flags().BoolVarP(&full, "full", "f", false, "full access mode, allow sensitive actions without approval")
	return rcmd
}

func resolveConfig(guard, full bool) (*Config, error) {
	switch {
	case guard && full:
		return nil, errors.New("cannot use both GuardMode and FullMode")
	case full:
		return &Config{Mode: approval.Full}, nil
	default:
		return &Config{Mode: approval.Guard}, nil
	}
}

func Execute(ctx context.Context) error {
	return rootCmd(ctx).ExecuteContext(ctx)
}
