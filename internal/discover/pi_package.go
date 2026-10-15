package discover

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/louisescher/hangar/internal/fsutil"
	"github.com/louisescher/hangar/internal/lockfile"
)

// A Pi manifest controls skills and prompts, including omitted or empty arrays.
// Agents are declared separately by @tintinweb/pi-subagents, not by Pi itself.
func discoverPiPackage(root string) ([]Skill, map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var pkg struct {
		Pi *struct {
			Skills    []string        `json:"skills"`
			Prompts   []string        `json:"prompts"`
			Subagents json.RawMessage `json:"subagents"`
		} `json:"pi"`
		Subagents json.RawMessage `json:"pi-subagents"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, nil, fmt.Errorf("parse package.json: %w", err)
	}
	declared := map[string]bool{}
	entries := map[string][]string{}
	agents := pkg.Subagents
	if pkg.Pi != nil {
		declared[lockfile.KindSkill], declared[lockfile.KindPrompt], declared[lockfile.KindAgent] = true, true, true
		entries[lockfile.KindSkill], entries[lockfile.KindPrompt] = pkg.Pi.Skills, pkg.Pi.Prompts
		if len(pkg.Pi.Subagents) > 0 {
			agents = pkg.Pi.Subagents
		}
	}
	if len(agents) > 0 && string(agents) != "null" {
		declared[lockfile.KindAgent] = true
		var paths []string
		if strings.HasPrefix(strings.TrimSpace(string(agents)), "[") {
			err = json.Unmarshal(agents, &paths)
		} else {
			var object struct {
				Agents []string `json:"agents"`
			}
			err = json.Unmarshal(agents, &object)
			paths = object.Agents
		}
		if err != nil {
			return nil, nil, fmt.Errorf("parse package subagents: %w", err)
		}
		entries[lockfile.KindAgent] = paths
	}
	var out []Skill
	for _, kind := range []string{lockfile.KindSkill, lockfile.KindPrompt, lockfile.KindAgent} {
		files, err := piPackageFiles(root, entries[kind], kind)
		if err != nil {
			return nil, nil, fmt.Errorf("package.json %s resources: %w", kind, err)
		}
		for _, file := range files {
			var sk Skill
			if kind == lockfile.KindSkill {
				sk, err = loadSkill(root, filepath.Dir(file))
			} else {
				sk, err = loadDefinition(root, file, kind)
			}
			if err != nil {
				return nil, nil, err
			}
			sk.PiPackage = true
			out = append(out, sk)
		}
	}
	return out, declared, nil
}

func piPackageFiles(root string, entries []string, kind string) ([]string, error) {
	files := map[string]bool{}
	for _, entry := range entries {
		if kind == lockfile.KindAgent && strings.ContainsAny(entry, "*?[{") {
			return nil, fmt.Errorf("pi.subagents.agents accepts files and directories, not globs: %q", entry)
		}
		if strings.HasPrefix(entry, "!") || (kind != lockfile.KindAgent && strings.HasPrefix(entry, "+")) || (kind != lockfile.KindAgent && strings.HasPrefix(entry, "-")) {
			continue
		}
		if entry == "" {
			return nil, fmt.Errorf("resource path must not be empty")
		}
		target := filepath.Join(root, entry)
		if filepath.IsAbs(entry) {
			return nil, fmt.Errorf("resource path %q must be relative to the package", entry)
		}
		if err := fsutil.MustWithinRoot(root, target); err != nil {
			return nil, err
		}
		paths := []string{target}
		if kind == lockfile.KindAgent && slices.ContainsFunc(entries, func(exclusion string) bool {
			return strings.HasPrefix(exclusion, "!") && filepath.Clean(exclusion[1:]) == filepath.Clean(entry)
		}) {
			continue
		}
		if strings.ContainsAny(entry, "*?[{") {
			var err error
			paths, err = doublestar.FilepathGlob(target, doublestar.WithFailOnIOErrors(), doublestar.WithNoFollow())
			if err != nil {
				return nil, err
			}
			// Pi's glob expansion does not discover dot-prefixed paths.
			paths = slices.DeleteFunc(paths, func(path string) bool {
				rel, _ := filepath.Rel(root, path)
				for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
					if strings.HasPrefix(part, ".") {
						return true
					}
				}
				return false
			})
		}
		for _, path := range paths {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil, err
			}
			resolvedRoot, err := filepath.EvalSymlinks(root)
			if err != nil {
				return nil, err
			}
			if err := fsutil.MustWithinRoot(resolvedRoot, resolved); err != nil {
				return nil, err
			}
			if err := collectPiPackageFiles(path, kind, files); err != nil {
				return nil, err
			}
		}
	}
	var out []string
	for file := range files {
		rel, _ := filepath.Rel(root, file)
		enabled := true
		// Pi applies exclusions, then exact force-includes, then force-excludes.
		for _, prefix := range []string{"!", "+", "-"} {
			if kind == lockfile.KindAgent && prefix != "!" {
				continue
			}
			for _, entry := range entries {
				if !strings.HasPrefix(entry, prefix) {
					continue
				}
				pattern := filepath.ToSlash(filepath.Clean(entry[1:]))
				match, err := piPackagePattern(pattern, filepath.ToSlash(rel), kind, prefix == "!")
				if err != nil {
					return nil, err
				}
				if match {
					enabled = prefix == "+"
				}
			}
		}
		if enabled {
			out = append(out, file)
		}
	}
	slices.Sort(out)
	return out, nil
}

func piPackagePattern(pattern, rel, kind string, glob bool) (bool, error) {
	if kind == lockfile.KindAgent {
		return pattern == rel, nil
	}
	candidates := []string{rel}
	if glob {
		candidates = append(candidates, filepath.Base(rel))
	}
	if kind == lockfile.KindSkill {
		parent := filepath.ToSlash(filepath.Dir(rel))
		candidates = append(candidates, parent)
		if glob {
			candidates = append(candidates, filepath.Base(parent))
		}
	}
	for _, candidate := range candidates {
		if !glob {
			if candidate == pattern {
				return true, nil
			}
			continue
		}
		match, err := doublestar.Match(pattern, candidate)
		if err != nil || match {
			return match, err
		}
	}
	return false, nil
}

func collectPiPackageFiles(path, kind string, files map[string]bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		if kind == lockfile.KindSkill && filepath.Base(path) != skillFile {
			return fmt.Errorf("%s: Hangar requires a SKILL.md directory; standalone Pi skill files are unsupported", path)
		}
		if filepath.Ext(path) != ".md" {
			return fmt.Errorf("%s: expected a Markdown file", path)
		}
		files[path] = true
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: expected a regular file or directory", path)
	}
	return filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != path && (kind == lockfile.KindAgent || strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if kind == lockfile.KindSkill && fileExists(filepath.Join(p, skillFile)) {
				files[filepath.Join(p, skillFile)] = true
				return filepath.SkipDir
			}
			return nil
		}
		if kind != lockfile.KindSkill && d.Type().IsRegular() && !strings.HasPrefix(d.Name(), ".") && filepath.Ext(p) == ".md" {
			files[p] = true
		}
		return nil
	})
}
