package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/louisescher/hangar/internal/lockfile"
	"github.com/louisescher/hangar/internal/rules"
)

func (s Skill) Type() string {
	if s.IsRef {
		return lockfile.KindRef
	}
	if s.Kind == "" {
		return lockfile.KindSkill
	}
	return s.Kind
}
func (s Skill) Key() string { return s.Type() + ":" + s.Name }

// IsRuleFile distinguishes Codex command policy files from instruction bundles.
func (s Skill) IsRuleFile() bool {
	return s.Type() == lockfile.KindRule && filepath.Ext(s.AbsPath) == ".rules"
}

// DiscoverContent extends skill discovery with native definitions in agents/,
// prompts/ or commands/, and one bundle per outermost rules/ directory.
func DiscoverContent(root string) ([]Skill, error) {
	out, err := DiscoverSkills(root)
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && strings.HasPrefix(d.Name(), ".") && d.Name() != ".pi" && d.Name() != ".codex" && d.Name() != ".cursor" {
				return filepath.SkipDir
			}
			if fileExists(filepath.Join(p, "SKILL.md")) {
				// The existing scan skips dot-directories; include native skills
				// encountered under the three harness directories handled here.
				rel, _ := filepath.Rel(root, p)
				slash := "/" + filepath.ToSlash(rel) + "/"
				native := strings.Contains(slash, "/.pi/") || strings.Contains(slash, "/.codex/") || strings.Contains(slash, "/.cursor/")
				found := false
				for _, sk := range out {
					if sk.AbsPath == p {
						found = true
						break
					}
				}
				if native && !found {
					sk, err := loadSkill(root, p)
					if err != nil {
						return err
					}
					out = append(out, sk)
				}
				return filepath.SkipDir
			}
			if d.Name() == "rules" {
				rel, _ := filepath.Rel(root, p)
				name := strings.Trim(strings.ReplaceAll(filepath.ToSlash(rel), "/", "-"), ".-")
				if name == "" {
					name = "rules"
				}
				hasPolicy, hasMarkdown := false, false
				err := filepath.WalkDir(p, func(file string, child os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !child.Type().IsRegular() {
						return nil
					}
					if rules.IsMarkdown(child.Name()) {
						hasMarkdown = true
					}
					if filepath.Ext(file) == ".rules" {
						hasPolicy = true
						policyRel, _ := filepath.Rel(root, file)
						policyName := strings.TrimSuffix(child.Name(), ".rules")
						out = append(out, Skill{Name: policyName, Kind: lockfile.KindRule, AbsPath: file, RelPath: filepath.ToSlash(policyRel), Description: "Codex command policy (.rules)"})
					}
					return nil
				})
				if err != nil {
					return err
				}
				if hasMarkdown || !hasPolicy {
					out = append(out, Skill{Name: name, Kind: lockfile.KindRule, AbsPath: p, RelPath: filepath.ToSlash(rel), Description: "Rules bundle (including nested directories)"})
				}
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		parent := filepath.Base(filepath.Dir(p))
		kind := ""
		ext := filepath.Ext(p)
		if parent == "agents" && (ext == ".md" || ext == ".toml") {
			kind = lockfile.KindAgent
		}
		if (parent == "prompts" || parent == "commands") && ext == ".md" {
			kind = lockfile.KindPrompt
		}
		if kind == "" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fm, _ := SplitFrontmatter(data)
		meta := parseMeta(fm)
		if ext == ".toml" {
			var agent struct {
				Name        string
				Description string
			}
			if _, err := toml.Decode(string(data), &agent); err != nil {
				return fmt.Errorf("parse agent %s: %w", p, err)
			}
			meta["name"], meta["description"] = agent.Name, agent.Description
		}
		name := strings.TrimSuffix(d.Name(), ext)
		if kind == lockfile.KindAgent && meta["name"] != "" {
			name = meta["name"]
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, Skill{Name: name, Kind: kind, AbsPath: p, RelPath: filepath.ToSlash(rel), Description: meta["description"], FrontmatterRaw: strings.TrimSpace(string(fm))})
		return nil
	})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, s := range out {
		if seen[s.Key()] {
			return nil, fmt.Errorf("duplicate content %s in source; select a narrower source path", s.Key())
		}
		seen[s.Key()] = true
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out, nil
}

func ReadContent(s Skill) ([]byte, error) {
	switch s.Type() {
	case lockfile.KindSkill:
		return os.ReadFile(filepath.Join(s.AbsPath, "SKILL.md"))
	case lockfile.KindRef:
		return os.ReadFile(filepath.Join(s.AbsPath, "REFERENCE.md"))
	case lockfile.KindRule:
		if s.IsRuleFile() {
			return os.ReadFile(s.AbsPath)
		}
		return rules.ContentFor(s.AbsPath, "")
	default:
		return os.ReadFile(s.AbsPath)
	}
}
