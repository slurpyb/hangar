package cmd

import (
	"fmt"
	"github.com/louisescher/hangar/internal/rules"
	"github.com/spf13/cobra"
)

func newCompileRulesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compile-rules <rules-directory> <output-path>",
		Short: "Combine Markdown rules into a managed instruction file",
		Long:  "Recursively concatenate .md rules in relative-path order. Output must be outside\nthe rules directory, named APPEND_SYSTEM.md, AGENTS.md, or AGENTS.append.md.\nUse .pi/APPEND_SYSTEM.md for Pi, or AGENTS.md for Codex. AGENTS.append.md\nrequires explicit harness configuration. Existing unmanaged files are refused.",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			if err := rules.Compile(args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "compiled rules to %s\n", args[1])
			return nil
		},
	}
}
