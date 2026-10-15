package cmd

import (
	"fmt"
	"strings"

	"github.com/louisescher/hangar/internal/config"
	"github.com/louisescher/hangar/internal/engine"
	"github.com/louisescher/hangar/internal/present"
	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Save and re-apply named sets of content and Pi packages",
		Long: "Profiles capture installed skills, references, agents, rules, prompts, and\n" +
			"native Pi packages so you can re-apply them\n" +
			"elsewhere. They are stored under your config dir (~/.config/hangar/profiles).",
	}
	cmd.AddCommand(newProfileSaveCmd(), newProfileApplyCmd(), newProfileListCmd(), newProfileShowCmd(), newProfileRemoveCmd())
	return cmd
}

func newProfileSaveCmd() *cobra.Command {
	var global bool
	var contentType string
	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save the current install as a named profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			eng := engine.New()
			profile, err := eng.CaptureProfile(args[0], engine.InstallOptions{Global: global, Type: contentType})
			if err != nil {
				return err
			}
			if profile.Count() == 0 {
				return fmt.Errorf("nothing installed to save")
			}
			if err := config.SaveProfile(profile); err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "saved profile %q (%s)\n", args[0], profile.Summary())
			return nil
		},
	}
	cmd.Flags().BoolVarP(&global, "global", "g", false, "save from the global (~/) install")
	cmd.Flags().StringVar(&contentType, "type", "", "save only skill, ref, agent, rule, prompt, or package entries")
	return cmd
}

func newProfileApplyCmd() *cobra.Command {
	var (
		contentType string
		piRulesFile string
		agentNames  []string
		global      bool
		asJSON      bool
		verbose     bool
		sec         securityFlags
	)
	cmd := &cobra.Command{
		Use:   "apply <name>",
		Short: "Restore content and native Pi packages from a saved profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			p, err := config.LoadProfile(args[0])
			if err != nil {
				return err
			}
			eng := engine.New()
			prog := newProgress(c.ErrOrStderr(), verbose)
			prog.start()
			rep, err := eng.RestoreProfile(c.Context(), p, engine.InstallOptions{
				Type:        contentType,
				PiRulesFile: piRulesFile,
				Agents:      agentNames,
				Global:      global,
				Security:    sec.sanitizeOpts(),
				OnProgress:  prog.event,
			})
			prog.finish()
			if err != nil {
				return err
			}
			return outputReport(c, rep, &sec, asJSON)
		},
	}
	cmd.Flags().StringArrayVarP(&agentNames, "agent", "a", nil, "override saved harnesses (repeatable); default: saved targets, or auto-detect for older entries")
	cmd.Flags().StringVar(&contentType, "type", "", "restore only skill, ref, agent, rule, prompt, or package entries")
	cmd.Flags().StringVar(&piRulesFile, "pi-rules-file", "", "override saved Pi rules destination: APPEND_SYSTEM.md, AGENTS.md, or AGENTS.override.md")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "install into the global (~/) location")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print progress")
	sec.register(cmd)
	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Browse saved profiles (interactive) or list them",
		Long: "On a terminal this opens an interactive browser: pick a profile, inspect\n" +
			"its content and packages, apply it, or remove individual entries. Piped it prints.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if interactive(false) {
				return runProfilesTUI(c)
			}
			names, err := config.ListProfiles()
			if err != nil {
				return err
			}
			w := c.OutOrStdout()
			if len(names) == 0 {
				fmt.Fprintln(w, "no saved profiles")
				return nil
			}
			for _, n := range names {
				if p, err := config.LoadProfile(n); err == nil {
					fmt.Fprintf(w, "  %-24s %s\n", n, p.Summary())
				} else {
					fmt.Fprintf(w, "  %s\n", n)
				}
			}
			return nil
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	var asJSON bool
	var contentType string
	cmd := &cobra.Command{
		Use: "show <name>", Aliases: []string{"info"},
		Short: "Inspect a profile's typed entries and saved settings", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			p, err := config.LoadProfile(args[0])
			if err != nil {
				return err
			}
			p, err = p.Filter(contentType)
			if err != nil {
				return err
			}
			if asJSON {
				return present.WriteJSON(c.OutOrStdout(), present.Profile(p))
			}
			w := c.OutOrStdout()
			fmt.Fprintf(w, "%s (%s)\n", p.Name, p.Summary())
			for _, e := range p.Skills {
				fmt.Fprintf(w, "  %s  %s\n", e.Key(), e.Source)
				if len(e.Harnesses) > 0 {
					fmt.Fprintf(w, "    harnesses: %s\n", strings.Join(e.Harnesses, ", "))
				}
				if e.Subpath != "" {
					fmt.Fprintf(w, "    path: %s\n", e.Subpath)
				}
				if e.PiPackage {
					fmt.Fprintln(w, "    declared in package.json")
				}
				if e.PiRulesFile != "" {
					fmt.Fprintf(w, "    Pi rules: %s\n", e.PiRulesFile)
				}
			}
			for _, pkg := range p.Packages {
				fmt.Fprintf(w, "  package:%s  Pi\n", pkg.Source)
				if len(pkg.Options) > 0 {
					fmt.Fprintf(w, "    options: %v\n", pkg.Options)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	cmd.Flags().StringVar(&contentType, "type", "", "show only skill, ref, agent, rule, prompt, or package entries")
	return cmd
}

func newProfileRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <name> [kind:name]",
		Aliases: []string{"rm"},
		Short:   "Delete a profile or remove one entry from it",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 2 {
				p, err := config.LoadProfile(args[0])
				if err != nil {
					return err
				}
				removed, err := p.RemoveEntry(args[1])
				if err != nil {
					return err
				}
				if !removed {
					return fmt.Errorf("no profile entry named %q", args[1])
				}
				if p.Count() == 0 {
					_, err = config.RemoveProfile(args[0])
				} else {
					err = config.SaveProfile(p)
				}
				if err != nil {
					return err
				}
				fmt.Fprintf(c.OutOrStdout(), "removed %s from profile %q\n", args[1], args[0])
				return nil
			}
			removed, err := config.RemoveProfile(args[0])
			if err != nil {
				return err
			}
			if !removed {
				return fmt.Errorf("no profile named %q", args[0])
			}
			fmt.Fprintf(c.OutOrStdout(), "removed profile %q\n", args[0])
			return nil
		},
	}
	return cmd
}
