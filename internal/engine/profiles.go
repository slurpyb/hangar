package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/louisescher/hangar/internal/config"
	"github.com/louisescher/hangar/internal/install"
	"github.com/louisescher/hangar/internal/lockfile"
)

func (e *Engine) CaptureProfile(name string, opt InstallOptions) (config.Profile, error) {
	base, err := resolveBaseDir(opt)
	if err != nil {
		return config.Profile{}, err
	}
	lf, err := lockfile.Load(base)
	if err != nil {
		return config.Profile{}, err
	}
	p := config.Profile{Name: name, Skills: lf.Skills}
	if opt.Type == "" || opt.Type == "package" {
		p.Packages, err = config.ReadPiPackages(base, opt.Global)
		if err != nil {
			return config.Profile{}, err
		}
	}
	return p.Filter(opt.Type)
}

// RestoreProfile uses the saved harnesses unless overridden. Whole Pi packages
// go through Pi's installer; selected content continues through Hangar's store.
func (e *Engine) RestoreProfile(ctx context.Context, profile config.Profile, opt InstallOptions) (install.Report, error) {
	p, err := profile.Filter(opt.Type)
	if err != nil {
		return install.Report{}, err
	}
	if p.Count() == 0 {
		return install.Report{}, fmt.Errorf("profile has no selected entries")
	}
	base, err := resolveBaseDir(opt)
	if err != nil {
		return install.Report{}, err
	}
	if _, err := install.PiRulesPath(opt.PiRulesFile, opt.Global); err != nil {
		return install.Report{}, err
	}
	pi := ""
	if len(p.Packages) > 0 {
		if len(opt.Agents) > 0 {
			agents, err := resolveAgents(opt.Agents, opt.Global)
			if err != nil {
				return install.Report{}, err
			}
			found := false
			for _, agent := range agents {
				if agent.Def.Name == "pi" {
					found = true
				}
			}
			if !found {
				return install.Report{}, fmt.Errorf("profile packages target Pi; include --agent pi or select a content --type")
			}
		}
		for _, pkg := range p.Packages {
			if strings.TrimSpace(pkg.Source) == "" {
				return install.Report{}, fmt.Errorf("Pi package source must not be empty")
			}
			if _, ok := pkg.Options["source"]; ok {
				return install.Report{}, fmt.Errorf("Pi package options must not override source")
			}
		}
		if _, err := config.ReadPiPackages(base, opt.Global); err != nil {
			return install.Report{}, err
		}
		pi, err = exec.LookPath("pi")
		if err != nil {
			return install.Report{}, fmt.Errorf("restoring profile packages requires pi on PATH: %w", err)
		}
	}
	rep := install.Report{}
	for i, pkg := range p.Packages {
		if opt.OnProgress != nil {
			opt.OnProgress(install.Event{Phase: "package", Name: pkg.Source, Index: i + 1, Total: len(p.Packages)})
		}
		args := []string{"install", pkg.Source}
		if !opt.Global {
			args = append(args, "--local", "--approve")
		}
		cmd := exec.CommandContext(ctx, pi, args...)
		cmd.Dir = base
		if output, err := cmd.CombinedOutput(); err != nil {
			return rep, fmt.Errorf("pi install %q: %w: %s", pkg.Source, err, strings.TrimSpace(string(output)))
		}
		if err := config.SetPiPackage(base, opt.Global, pkg); err != nil {
			return rep, err
		}
		rep.Packages = append(rep.Packages, pkg.Source)
	}
	if len(p.Skills) > 0 {
		content, err := e.ApplyProfile(ctx, p.Skills, opt)
		content.Packages = rep.Packages
		return content, err
	}
	return rep, nil
}
