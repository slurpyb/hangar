package install

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/louisescher/hangar/internal/agents"
	"github.com/louisescher/hangar/internal/discover"
	"github.com/louisescher/hangar/internal/fsutil"
	"github.com/louisescher/hangar/internal/link"
	"github.com/louisescher/hangar/internal/lockfile"
	"github.com/louisescher/hangar/internal/rules"
	"github.com/louisescher/hangar/internal/security/audit"
	"github.com/louisescher/hangar/internal/security/sanitize"
)

// StorePath is the canonical location for a new content entry. Existing skills
// and references retain their original stores.
func StorePath(base string, e lockfile.Entry) string {
	dir := map[string]string{lockfile.KindAgent: "agents", lockfile.KindPrompt: "prompts", lockfile.KindRule: "rules"}[e.Kind]
	return filepath.Join(base, ".agents", dir, e.Name)
}

// contentTarget handles the native conventions of the three supported harnesses.
func contentTarget(req Request, sk discover.Skill, ag agents.Agent) (string, error) {
	var target string
	switch ag.Def.Name {
	case "codex":
		switch sk.Type() {
		case lockfile.KindAgent:
			if filepath.Ext(sk.AbsPath) != ".toml" {
				return "", fmt.Errorf("Codex agent definitions require native .toml files")
			}
			raw, err := os.ReadFile(sk.AbsPath)
			if err != nil {
				return "", err
			}
			var def struct {
				Name         string
				Description  string
				Instructions string `toml:"developer_instructions"`
			}
			if _, err := toml.Decode(string(raw), &def); err != nil {
				return "", fmt.Errorf("parse Codex agent: %w", err)
			}
			if def.Name == "" || def.Description == "" || def.Instructions == "" {
				return "", fmt.Errorf("Codex agent requires name, description, and developer_instructions")
			}
			target = filepath.Join(".codex", "agents", sk.Name+".toml")
		case lockfile.KindPrompt:
			if !req.Options.Global {
				return "", fmt.Errorf("Codex prompts are global-only; use --global")
			}
			target = filepath.Join(".codex", "prompts", sk.Name+".md")
		case lockfile.KindRule:
			if sk.IsRuleFile() {
				target = filepath.Join(".codex", "rules", sk.Name+".rules")
				break
			}
			if _, err := rules.Content(sk.AbsPath); err != nil {
				return "", err
			}
			target = "AGENTS.md"
			if req.Options.Global {
				target = ".codex/AGENTS.md"
			}
		}
	case "cursor":
		switch sk.Type() {
		case lockfile.KindAgent:
			if filepath.Ext(sk.AbsPath) != ".md" {
				return "", fmt.Errorf("Cursor agent definitions require native .md files with YAML frontmatter")
			}
			raw, err := os.ReadFile(sk.AbsPath)
			if err != nil {
				return "", err
			}
			fm, _ := discover.SplitFrontmatter(raw)
			if len(fm) == 0 {
				return "", fmt.Errorf("Cursor agent definitions require YAML frontmatter")
			}
			target = filepath.Join(".cursor", "agents", sk.Name+".md")
		case lockfile.KindPrompt:
			target = filepath.Join(".cursor", "commands", sk.Name+".md")
		case lockfile.KindRule:
			if sk.IsRuleFile() {
				return "", fmt.Errorf("Cursor rules require native .mdc files, not Codex .rules policies")
			}
			if req.Options.Global {
				return "", fmt.Errorf("Cursor file-based rules are project-only; user rules are managed in Cursor settings")
			}
			if _, err := rules.Files(sk.AbsPath, ".mdc"); err != nil {
				return "", fmt.Errorf("Cursor rules require native .mdc files: %w", err)
			}
			target = filepath.Join(".cursor", "rules", sk.Name)
		}
	case "pi":
		prefix := ".pi"
		if req.Options.Global {
			prefix = ".pi/agent"
		}
		switch sk.Type() {
		case lockfile.KindAgent:
			if filepath.Ext(sk.AbsPath) != ".md" {
				return "", fmt.Errorf("Pi agents via @tintinweb/pi-subagents require Markdown .md definitions")
			}
			if strings.Contains(sk.Name, ":") {
				return "", fmt.Errorf("@tintinweb/pi-subagents agent names cannot contain ':'")
			}
			target = filepath.Join(prefix, "agents", sk.Name+".md")
		case lockfile.KindPrompt:
			target = filepath.Join(prefix, "prompts", sk.Name+".md")
		case lockfile.KindRule:
			if sk.IsRuleFile() {
				return "", fmt.Errorf("Pi rules compilation requires a Markdown directory, not a Codex .rules policy")
			}
			if _, err := rules.Content(sk.AbsPath); err != nil {
				return "", err
			}
			target = filepath.Join(prefix, "APPEND_SYSTEM.md")
		}
	}
	if target == "" {
		return "", fmt.Errorf("%s does not support %s content in this scope/format", ag.Def.Display, sk.Type())
	}
	return target, nil
}

func destinationPath(base, dst string) string {
	if filepath.IsAbs(dst) {
		return dst
	}
	return filepath.Join(base, dst)
}

// preflightContent validates all new combinations and ownership before any
// install writes, including collisions between selected rule bundles.
func preflightContent(req Request, lf *lockfile.Lockfile) error {
	claimed := map[string]string{}
	for _, sk := range req.Skills {
		if sk.Type() == lockfile.KindSkill {
			continue
		}
		if len(req.Agents) == 0 {
			return fmt.Errorf("select a target harness with --agent for %s", sk.Key())
		}
		store := StorePath(req.BaseDir, lockfile.Entry{Name: sk.Name, Kind: sk.Type()})
		if err := fsutil.MustWithinRoot(filepath.Join(req.BaseDir, ".agents", sk.Type()+"s"), store); err != nil {
			return err
		}
		for _, ag := range req.Agents {
			dst, err := contentTarget(req, sk, ag)
			if err != nil {
				return fmt.Errorf("%s: %w", sk.Key(), err)
			}
			abs := destinationPath(req.BaseDir, dst)
			if sk.Type() == lockfile.KindRule {
				if err := rules.ValidateOutput(sk.AbsPath, abs); err != nil {
					return err
				}
			}
			if key, ok := claimed[dst]; ok && key != sk.Key() {
				return fmt.Errorf("%s and %s share destination %s; install one rules bundle per harness", key, sk.Key(), dst)
			}
			claimed[dst] = sk.Key()
			owned := false
			for _, en := range lf.Skills {
				for _, old := range en.Destinations {
					if old == dst {
						if en.Key() != sk.Key() {
							return fmt.Errorf("destination %s is already managed by %s", dst, en.Key())
						}
						owned = true
					}
				}
			}
			if fsutil.Exists(abs) && !owned {
				return fmt.Errorf("refusing to overwrite unmanaged content %s", abs)
			}
		}
	}
	return nil
}

func installContent(req Request, lf *lockfile.Lockfile, sk discover.Skill, op audit.Operation) (SkillResult, error) {
	entry := lockfile.Entry{Name: sk.Name, Kind: sk.Type(), Source: req.Meta.Source, Subpath: path.Join(req.Meta.Subpath, sk.RelPath), Ref: req.Meta.Ref, SHA: req.Meta.SHA, Version: req.Meta.Version, Pinned: req.Meta.Pinned, InstalledAt: req.Now}
	store := StorePath(req.BaseDir, entry)
	if sk.Type() != lockfile.KindRule || sk.IsRuleFile() {
		entry.Manifest = filepath.Base(sk.AbsPath)
	}
	old, _ := discover.ReadContent(storedContent(req.BaseDir, entry, sk))
	if sk.Type() == lockfile.KindRule && !sk.IsRuleFile() {
		if err := link.Copy(sk.AbsPath, store); err != nil {
			return SkillResult{}, err
		}
		if err := sanitize.SkillDir(store, req.Options.Security); err != nil {
			return SkillResult{}, err
		}
		if err := filepath.WalkDir(store, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".mdc") {
				return nil
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(p, []byte(sanitize.Skill(string(raw), req.Options.Security)), 0644)
		}); err != nil {
			return SkillResult{}, err
		}
	} else {
		entry.Manifest = filepath.Base(sk.AbsPath)
		raw, err := os.ReadFile(sk.AbsPath)
		if err != nil {
			return SkillResult{}, err
		}
		if err := fsutil.AtomicWriteFile(filepath.Join(store, entry.Manifest), []byte(sanitize.Skill(string(raw), req.Options.Security)), 0o644); err != nil {
			return SkillResult{}, err
		}
	}
	sr := SkillResult{Name: sk.Name, Kind: sk.Type(), FailedAgents: map[string]string{}}
	installed := storedContent(req.BaseDir, entry, sk)
	for _, ag := range req.Agents {
		dst, err := contentTarget(req, sk, ag)
		if err != nil {
			return sr, err
		}
		abs := destinationPath(req.BaseDir, dst)
		if sk.Type() == lockfile.KindRule && !sk.IsRuleFile() && ag.Def.Name != "cursor" {
			data, err := rules.Content(store)
			if err != nil {
				return sr, err
			}
			if err := fsutil.AtomicWriteFile(abs, data, 0o644); err != nil {
				return sr, err
			}
		} else if req.Options.Global {
			data, err := os.ReadFile(installed.AbsPath)
			if err != nil {
				return sr, err
			}
			if err := fsutil.AtomicWriteFile(abs, data, 0644); err != nil {
				return sr, err
			}
		} else {
			if err := link.Symlink(installed.AbsPath, abs); err != nil {
				return sr, err
			}
		}
		entry.Destinations = append(entry.Destinations, dst)
		entry.Harnesses = append(entry.Harnesses, ag.Def.Name)
		sr.InstalledAgents = append(sr.InstalledAgents, ag.Def.Name)
	}
	// Keep destinations from previous installs into other harnesses.
	if prev, ok := lf.Find(entry.Key()); ok {
		for i, dst := range prev.Destinations {
			found := false
			for _, current := range entry.Destinations {
				if current == dst {
					found = true
				}
			}
			if !found {
				entry.Destinations = append(entry.Destinations, dst)
				entry.Harnesses = append(entry.Harnesses, prev.Harnesses[i])
			}
		}
	}
	lf.Upsert(entry)
	data, err := discover.ReadContent(installed)
	if err != nil {
		return sr, err
	}
	content := string(data)
	change := audit.Change{Name: sk.Key(), Kind: audit.Kind(sk.Type()), Source: req.Meta.Source, Ref: req.Meta.Ref, SHA: req.Meta.SHA, Operation: op}
	if op == audit.OpUpdate && len(old) > 0 {
		d := audit.UnifiedDiff(sk.Key(), string(old), content)
		change.Diff = &d
	} else {
		change.Content = &content
	}
	req.Audit.AddChange(change)
	return sr, nil
}

func storedContent(base string, entry lockfile.Entry, source discover.Skill) discover.Skill {
	source.AbsPath = StorePath(base, entry)
	if entry.Kind != lockfile.KindRule || entry.Manifest != "" {
		source.AbsPath = filepath.Join(source.AbsPath, entry.Manifest)
	}
	return source
}
