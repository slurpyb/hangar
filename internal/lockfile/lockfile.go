// Package lockfile reads and writes Hangar's TOML lockfile,
// <baseDir>/.agents/hangar.lock, which records every installed skill and
// reference so installs are reproducible.
package lockfile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/louisescher/hangar/internal/fsutil"
)

// SchemaVersion is the current lockfile schema version.
const SchemaVersion = 1

// Kind values for a lockfile entry.
const (
	KindSkill  = "skill"
	KindRef    = "ref"
	KindAgent  = "agent"
	KindRule   = "rule"
	KindPrompt = "prompt"
)

// RelPath is the lockfile location relative to a base directory.
const RelPath = ".agents/hangar.lock"

// Entry records a single installed skill or reference.
type Entry struct {
	Name         string    `toml:"name"`
	Manifest     string    `toml:"manifest,omitempty"`
	Harnesses    []string  `toml:"harnesses,omitempty"`
	Destinations []string  `toml:"destinations,omitempty"`
	Source       string    `toml:"source"`            // owner/repo, file://path, or npm:pkg
	Subpath      string    `toml:"subpath,omitempty"` // crawl subpath within the source
	Ref          string    `toml:"ref,omitempty"`     // branch/tag (GitHub)
	SHA          string    `toml:"sha,omitempty"`     // resolved commit SHA (GitHub)
	Version      string    `toml:"version,omitempty"` // package version (npm)
	File         string    `toml:"file,omitempty"`    // reference file within an npm package
	InstalledAt  time.Time `toml:"installed_at"`
	Pinned       bool      `toml:"pinned"`
	Kind         string    `toml:"kind"` // KindSkill | KindRef
}

// Lockfile is the parsed lockfile plus the base directory it belongs to.
type Compilation struct {
	Output string `toml:"output"`
	Source string `toml:"source"`
}

type Lockfile struct {
	Compilations []Compilation `toml:"compilations,omitempty"`
	Version      int           `toml:"version"`
	Skills       []Entry       `toml:"skills"`

	baseDir string `toml:"-"`
}

// Path returns the absolute lockfile path for baseDir.
func Path(baseDir string) string {
	return filepath.Join(baseDir, RelPath)
}

// Load reads the lockfile under baseDir. A missing file yields an empty
// lockfile (not an error).
func Load(baseDir string) (*Lockfile, error) {
	lf := &Lockfile{Version: SchemaVersion, baseDir: baseDir}
	data, err := os.ReadFile(Path(baseDir))
	if err != nil {
		if os.IsNotExist(err) {
			return lf, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, lf); err != nil {
		return nil, fmt.Errorf("parse lockfile: %w", err)
	}
	if lf.Version == 0 {
		lf.Version = SchemaVersion
	}
	lf.baseDir = baseDir
	return lf, nil
}

// Save writes the lockfile atomically, with entries sorted by name for stable
// diffs.
func (l *Lockfile) Save() error {
	if l.Version == 0 {
		l.Version = SchemaVersion
	}
	sort.Slice(l.Skills, func(i, j int) bool { return l.Skills[i].Key() < l.Skills[j].Key() })

	var buf bytes.Buffer
	buf.WriteString("# hangar lockfile — managed by `hangar`; do not edit by hand.\n")
	if err := toml.NewEncoder(&buf).Encode(l); err != nil {
		return fmt.Errorf("encode lockfile: %w", err)
	}
	return fsutil.AtomicWriteFile(Path(l.baseDir), buf.Bytes(), 0o644)
}

// Key identifies an entry independently of other content types.
func (e Entry) Key() string { return e.Kind + ":" + e.Name }

// Lookup accepts a typed key or a unique bare name.
func (l *Lockfile) Lookup(name string) (Entry, bool, error) {
	var matches []Entry
	for _, e := range l.Skills {
		if e.Key() == name {
			return e, true, nil
		}
		if e.Name == name {
			matches = append(matches, e)
		}
	}
	if len(matches) > 1 {
		return Entry{}, false, fmt.Errorf("ambiguous content name %q; use kind:name (skill, ref, agent, rule, prompt)", name)
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	return Entry{}, false, nil
}

func (l *Lockfile) Find(name string) (Entry, bool) {
	e, ok, _ := l.Lookup(name)
	return e, ok
}

// Upsert replaces an entry with the same kind and name.
func (l *Lockfile) Upsert(e Entry) {
	for i, existing := range l.Skills {
		if existing.Key() == e.Key() {
			l.Skills[i] = e
			return
		}
	}
	l.Skills = append(l.Skills, e)
}

// SetPinned sets the pinned flag on the named entry, reporting whether one
// existed.
func (l *Lockfile) SetPinned(name string, pinned bool) bool {
	entry, ok := l.Find(name)
	if !ok {
		return false
	}
	name = entry.Key()
	for i := range l.Skills {
		if l.Skills[i].Key() == name {
			l.Skills[i].Pinned = pinned
			return true
		}
	}
	return false
}

// Remove deletes the entry with the given name, reporting whether one existed.
func (l *Lockfile) Remove(name string) bool {
	entry, ok := l.Find(name)
	if !ok {
		return false
	}
	name = entry.Key()
	for i, e := range l.Skills {
		if e.Key() == name {
			l.Skills = append(l.Skills[:i], l.Skills[i+1:]...)
			return true
		}
	}
	return false
}

// Refs returns the reference entries (kind == ref).
func (l *Lockfile) Refs() []Entry {
	var out []Entry
	for _, e := range l.Skills {
		if e.Kind == KindRef {
			out = append(out, e)
		}
	}
	return out
}
