// Package rules concatenates authored Markdown rules without rewriting them.
package rules

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/louisescher/hangar/internal/fsutil"
	"github.com/louisescher/hangar/internal/lockfile"
)

// Files collects regular files with the given extension (empty selects .md and
// .mdc for preview), including nested rules
// directories. Paths are sorted relative to the outermost directory passed in.
func Files(dir, ext string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("rules directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("rules source %q is not a directory", dir)
	}
	var files []string
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() && (filepath.Ext(d.Name()) == ext || ext == "" && IsMarkdown(d.Name())) {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		if ext == "" {
			return nil, fmt.Errorf("rules directory %q contains no Markdown files", dir)
		}
		return nil, fmt.Errorf("rules directory %q contains no %s files", dir, ext)
	}
	return files, nil
}

// Content preserves each source file verbatim, adding a blank line between files.
func Content(dir string) ([]byte, error) { return ContentFor(dir, ".md") }

func ContentFor(dir, ext string) ([]byte, error) {
	files, err := Files(dir, ext)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	for i, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			if !bytes.HasSuffix(out.Bytes(), []byte("\n")) {
				out.WriteByte('\n')
			}
			out.WriteByte('\n')
		}
		out.Write(data)
	}
	return out.Bytes(), nil
}

// Compile writes a complete replacement, never appending to an existing output.
// Ownership is recorded in the existing Hangar lockfile beside the output.
func Compile(dir, output string) error {
	switch filepath.Base(output) {
	case "APPEND_SYSTEM.md", "AGENTS.md", "AGENTS.append.md":
	default:
		return fmt.Errorf("output must be APPEND_SYSTEM.md, AGENTS.md, or AGENTS.append.md")
	}
	src, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	dst, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	if within, err := fsutil.WithinRoot(src, dst); err != nil {
		return err
	} else if within {
		return fmt.Errorf("rules output must be outside the source rules directory")
	}
	// Resolve existing ancestors as well, so an output through a directory symlink
	// cannot become part of the next input walk.
	parent, err := filepath.EvalSymlinks(filepath.Dir(dst))
	if err == nil {
		realSrc, e := filepath.EvalSymlinks(src)
		if e != nil {
			return e
		}
		if within, _ := fsutil.WithinRoot(realSrc, filepath.Join(parent, filepath.Base(dst))); within {
			return fmt.Errorf("rules output must be outside the source rules directory")
		}
	}
	data, err := Content(src)
	if err != nil {
		return err
	}
	lf, err := lockfile.Load(filepath.Dir(dst))
	if err != nil {
		return err
	}
	managed := false
	for _, c := range lf.Compilations {
		if c.Output == filepath.Base(dst) {
			managed = true
		}
	}
	if fsutil.Exists(dst) && !managed {
		return fmt.Errorf("refusing to overwrite unmanaged instruction file %s", dst)
	}
	if err := fsutil.AtomicWriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write compiled rules: %w", err)
	}
	if !managed {
		lf.Compilations = append(lf.Compilations, lockfile.Compilation{Output: filepath.Base(dst), Source: src})
	}
	if err := lf.Save(); err != nil {
		return fmt.Errorf("record compiled rules: %w", err)
	}
	return nil
}

// ValidateOutput keeps installed instruction files outside their source tree.
func ValidateOutput(dir, output string) error {
	within, err := fsutil.WithinRoot(dir, output)
	if err != nil {
		return err
	}
	if within {
		return fmt.Errorf("rules output %s must be outside source %s", output, dir)
	}
	return nil
}

// IsMarkdown reports the authored rule formats Hangar discovers.
func IsMarkdown(name string) bool {
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".mdc")
}
