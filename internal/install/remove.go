package install

import (
	"os"
	"path/filepath"

	"github.com/louisescher/hangar/internal/agents"
	"github.com/louisescher/hangar/internal/lockfile"
)

// Remove deletes the named skill/reference from each target agent, the canonical
// store, and the lockfile, then rebuilds the references block. It reports
// whether a lockfile entry was found and removed.
func Remove(baseDir, name string, agts []agents.Agent, global bool) (bool, error) {
	lf, err := lockfile.Load(baseDir)
	if err != nil {
		return false, err
	}
	entry, ok, err := lf.Lookup(name)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	if entry.Kind == lockfile.KindAgent || entry.Kind == lockfile.KindRule || entry.Kind == lockfile.KindPrompt {
		for _, dst := range entry.Destinations {
			target := destinationPath(baseDir, dst)
			if !global && (entry.Kind != lockfile.KindRule || filepath.Base(filepath.Dir(target)) == "rules") {
				removeAgentEntry(target, false)
				continue
			}
			if err := os.RemoveAll(target); err != nil {
				return false, err
			}
		}
		if err := os.RemoveAll(StorePath(baseDir, entry)); err != nil {
			return false, err
		}
		lf.Remove(entry.Key())
		if err := lf.Save(); err != nil {
			return false, err
		}
		_, err := rebuildReferences(baseDir, lf)
		return true, err
	}
	name = entry.Name
	canonicalRoot := filepath.Join(baseDir, ".agents", "skills")
	linkBase := baseDir
	if global {
		linkBase = ""
	}

	if entry.Kind == lockfile.KindSkill {
		for _, ag := range agts {
			agentSkillsDir := filepath.Join(linkBase, ag.InstallPath)
			if filepath.Clean(agentSkillsDir) == filepath.Clean(canonicalRoot) {
				continue // canonical store, handled below
			}
			removeAgentEntry(filepath.Join(agentSkillsDir, name), global)
		}

	}
	if entry.Kind == lockfile.KindRef {
		_ = os.RemoveAll(filepath.Join(baseDir, ".agents", "references", name))
	} else {
		_ = os.RemoveAll(filepath.Join(canonicalRoot, name))
	}

	lf.Remove(entry.Key())
	if err := lf.Save(); err != nil {
		return false, err
	}
	if _, err := rebuildReferences(baseDir, lf); err != nil {
		return false, err
	}
	return true, nil
}

// removeAgentEntry removes a per-agent install: always a symlink for local
// installs, a copied directory for global installs. Real files we did not
// create are left untouched.
func removeAgentEntry(target string, global bool) {
	fi, err := os.Lstat(target)
	if err != nil {
		return
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		_ = os.Remove(target)
	case global && fi.IsDir():
		_ = os.RemoveAll(target)
	}
}
