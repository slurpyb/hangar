package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/louisescher/hangar/internal/fsutil"
	"github.com/louisescher/hangar/internal/lockfile"
)

// Profile is a named, portable set of installed entries that can be re-applied
// to another project.
type Profile struct {
	Name     string           `toml:"name" json:"name"`
	Skills   []lockfile.Entry `toml:"skills,omitempty" json:"skills"`
	Packages []PiPackage      `toml:"packages,omitempty" json:"packages,omitempty"`
}

// PiPackage is a native Pi settings.json package declaration. Options preserve
// object-form resource filters, including the distinction between [] and absent.
type PiPackage struct {
	Source  string         `toml:"source" json:"source"`
	Options map[string]any `toml:"options,omitempty" json:"options,omitempty"`
}

func (p Profile) Count() int { return len(p.Skills) + len(p.Packages) }

func (p Profile) Summary() string {
	counts := map[string]int{}
	for _, e := range p.Skills {
		counts[e.Kind]++
	}
	counts["package"] = len(p.Packages)
	var parts []string
	for _, kind := range []string{"skill", "ref", "agent", "rule", "prompt", "package"} {
		if n := counts[kind]; n > 0 {
			label := kind
			if n != 1 {
				label += "s"
			}
			parts = append(parts, fmt.Sprintf("%d %s", n, label))
		}
	}
	return strings.Join(parts, ", ")
}

// Filter selects one content type without changing the saved profile.
func (p Profile) Filter(kind string) (Profile, error) {
	if kind == "" {
		return p, nil
	}
	switch kind {
	case lockfile.KindSkill, lockfile.KindRef, lockfile.KindAgent, lockfile.KindRule, lockfile.KindPrompt, "package":
	default:
		return Profile{}, fmt.Errorf("unknown profile type %q; choose skill, ref, agent, rule, prompt, or package", kind)
	}
	selected := Profile{Name: p.Name}
	for _, entry := range p.Skills {
		if entry.Kind == kind {
			selected.Skills = append(selected.Skills, entry)
		}
	}
	if kind == "package" {
		selected.Packages = p.Packages
	}
	return selected, nil
}

// RemoveEntry removes one typed entry, accepting bare names only when unique.
func (p *Profile) RemoveEntry(name string) (bool, error) {
	var matches []string
	for _, e := range p.Skills {
		if e.Key() == name {
			matches = []string{name}
			break
		}
		if e.Name == name {
			matches = append(matches, e.Key())
		}
	}
	for _, pkg := range p.Packages {
		key := "package:" + pkg.Source
		if key == name {
			matches = []string{key}
			break
		}
		if pkg.Source == name {
			matches = append(matches, key)
		}
	}
	if len(matches) > 1 {
		return false, fmt.Errorf("ambiguous profile entry %q; use kind:name", name)
	}
	if len(matches) == 0 {
		return false, nil
	}
	key := matches[0]
	for i, e := range p.Skills {
		if e.Key() == key {
			p.Skills = append(p.Skills[:i], p.Skills[i+1:]...)
			return true, nil
		}
	}
	for i, pkg := range p.Packages {
		if "package:"+pkg.Source == key {
			p.Packages = append(p.Packages[:i], p.Packages[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

var profileNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// ProfilesDir returns the directory holding saved profiles.
func ProfilesDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles"), nil
}

func profilePath(name string) (string, error) {
	if !profileNameRe.MatchString(name) {
		return "", fmt.Errorf("invalid profile name %q (use letters, digits, '.', '_', '-')", name)
	}
	dir, err := ProfilesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".toml"), nil
}

// SaveProfile writes a profile atomically.
func SaveProfile(p Profile) error {
	path, err := profilePath(p.Name)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("# hangar profile\n")
	if err := toml.NewEncoder(&buf).Encode(p); err != nil {
		return fmt.Errorf("encode profile: %w", err)
	}
	return fsutil.AtomicWriteFile(path, buf.Bytes(), 0o644)
}

// LoadProfile reads a saved profile by name.
func LoadProfile(name string) (Profile, error) {
	path, err := profilePath(name)
	if err != nil {
		return Profile{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Profile{}, fmt.Errorf("no profile named %q", name)
		}
		return Profile{}, err
	}
	var p Profile
	if err := toml.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("parse profile %q: %w", name, err)
	}
	if p.Name == "" {
		p.Name = name
	}
	return p, nil
}

// ListProfiles returns the names of all saved profiles, sorted.
func ListProfiles() ([]string, error) {
	dir, err := ProfilesDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".toml"))
	}
	sort.Strings(names)
	return names, nil
}

// RemoveProfile deletes a saved profile, reporting whether it existed.
func RemoveProfile(name string) (bool, error) {
	path, err := profilePath(name)
	if err != nil {
		return false, err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
