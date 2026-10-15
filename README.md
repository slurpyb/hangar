<img width="1480" height="766" alt="image" src="https://github.com/user-attachments/assets/b4523d3a-2422-41f7-916d-4ffa3cc153ea" />

---

# Hangar ✈

A TUI package manager for **Agent Skills**, references, agent definitions,
rules, and prompts. Existing skills work with Claude Code, Cursor, Copilot,
opencode, Codex, Gemini CLI, and ~50 other harnesses. Agents, rules, and prompts
use the native conventions of Pi, Codex, and Cursor where supported.

Hangar discovers content in **Git repositories**, **npm packages**, and local
directories, lets you select it in an interactive picker, and installs it into
your chosen harness applications.

Inspired by [`withastro/rosie`](https://github.com/withastro/rosie), with a
TUI-first workflow, subpath installs, real npm-registry fetching, a TOML
lockfile, and a `doctor`.

---

## Install

### Homebrew (macOS)

```sh
brew install louisescher/tap/hangar
```

### Scoop (Windows)

```sh
scoop bucket add louisescher https://github.com/louisescher/scoop-bucket
scoop install hangar
```

### Prebuilt binaries (Linux, macOS, Windows)

Download the archive for your platform from the
[latest release](https://github.com/louisescher/hangar/releases/latest), extract
it, and put the `hangar` binary on your `PATH`.

> [!NOTE]
> The prebuilt binaries are not code-signed. On macOS, Gatekeeper will
> block the first run — right-click the binary in Finder and choose **Open**, or
> run `xattr -d com.apple.quarantine ./hangar` from the extraction directory.
> On Windows, you may need to click **More info → Run anyway** in the SmartScreen prompt.

### From source

```sh
go install github.com/louisescher/hangar@latest
```

Or clone and build:

```sh
git clone https://github.com/louisescher/hangar && cd hangar
go build -o hangar .
```

## Agents, rules, and prompts

Hangar also manages installable **agent definitions**, **rules bundles**, and
**prompt templates**. `--agent` / `-a` selects the **harness application** (Pi,
Codex, or Cursor); an `agent` content item is a definition used by that
application. Hangar does not install the applications themselves.

### Supported formats and destinations

Project paths are relative to the install directory; `~` means the home
directory selected by `--global`. Skills still use the canonical
`.agents/skills/<name>/` store, with per-harness links locally and copies
globally. References remain in `.agents/references/<name>/REFERENCE.md` and
are linked from the existing managed instruction block.

| Harness | Content | Project destination | Global destination |
| --- | --- | --- | --- |
| Pi | Skill (`SKILL.md` directory) | `.pi/skills/<name>/` | `~/.pi/agent/skills/<name>/` |
| Pi | Agent (`.md`, via `@tintinweb/pi-subagents`) | `.pi/agents/<name>.md` | `~/.pi/agent/agents/<name>.md` |
| Pi | Rules (`.md` directory, compiled) | `.pi/APPEND_SYSTEM.md` (default), `AGENTS.md`, or `AGENTS.override.md` | Selected filename under `~/.pi/agent/` |
| Pi | Prompt (`.md`) | `.pi/prompts/<name>.md` | `~/.pi/agent/prompts/<name>.md` |
| Codex | Skill | `.agents/skills/<name>/` | `~/.agents/skills/<name>/` |
| Codex | Agent (native `.toml`) | `.codex/agents/<name>.toml` | `~/.codex/agents/<name>.toml` |
| Codex | Command policy (native `.rules`) | `.codex/rules/<name>.rules` | `~/.codex/rules/<name>.rules` |
| Codex | Instruction rules (`.md` directory, compiled) | `AGENTS.md` | `~/.codex/AGENTS.md` |
| Codex | Prompt (legacy `.md`) | Unsupported | `~/.codex/prompts/<name>.md` |
| Cursor | Skill | `.cursor/skills/<name>/` | `~/.cursor/skills/<name>/` |
| Cursor | Agent (`.md` with YAML frontmatter) | `.cursor/agents/<name>.md` | `~/.cursor/agents/<name>.md` |
| Cursor | Rules (native `.mdc`) | `.cursor/rules/<bundle>/` | Unsupported (user rules live in Cursor settings) |
| Cursor | Prompt / command (`.md`) | `.cursor/commands/<name>.md` | `~/.cursor/commands/<name>.md` |
| All three | Reference (existing npm documentation flow) | `.agents/references/` + instruction block | `~/.agents/references/` + instruction block |

The other existing harness targets retain skills/references support. New content
types fail with a clear error for unsupported harnesses, scopes, or formats.
Formats and prompt arguments are preserved; there is no translation between
Codex TOML agents and Cursor Markdown agents, or between prompt syntaxes.

These destinations follow [Pi subagents](https://github.com/tintinweb/pi-subagents#custom-agents),
[Pi configuration](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/configuration.md),
[Codex skills](https://learn.chatgpt.com/docs/build-skills),
[Codex agents](https://learn.chatgpt.com/docs/agent-configuration/subagents),
[Codex instructions](https://learn.chatgpt.com/docs/agent-configuration/agents-md),
[Codex command policies](https://learn.chatgpt.com/docs/agent-configuration/rules),
[Codex legacy prompts](https://learn.chatgpt.com/docs/custom-prompts),
[Cursor agents](https://cursor.com/docs/subagents),
[Cursor rules](https://cursor.com/docs/rules), and
[Cursor commands](https://cursor.com/changelog/1-6).

### Pi agent provider

Pi agents use **`@tintinweb/pi-subagents`**. Install the provider in Pi once:

```sh
pi install npm:@tintinweb/pi-subagents
```

Then install definitions with Hangar:

```sh
hangar install ./my-content --type agent -a pi --all
hangar info agent:reviewer
hangar update agent:reviewer
hangar remove agent:reviewer
```

Definitions are Markdown files with optional YAML frontmatter. Hangar preserves
provider fields such as `name`, `description`, `tools`, `model`, `thinking`, and
`max_turns`, plus the system prompt body, using the existing sanitization.
The provider uses frontmatter `name` when present and the filename otherwise.
Hangar writes `.pi/agents/` locally and `~/.pi/agent/agents/` globally; it
manages definitions separately from installing or configuring the provider.
TOML agent definitions are rejected for Pi. Verified against provider 0.22.0.

### Pi package manifests

Hangar reads resource declarations from a source root's `package.json`, including
for npm packages, Git sources, and local directories:

```json
{
  "name": "my-pi-content",
  "pi": {
    "skills": ["./resources/skills", "!./resources/skills/experimental"],
    "prompts": ["./resources/prompts/**/*.md", "!draft.md"],
    "subagents": {
      "agents": ["./resources/agents", "!./resources/agents/draft.md"]
    }
  }
}
```

```sh
hangar install npm:my-pi-content -a pi --all
hangar install ./my-pi-content -a pi --all
```

Skill and prompt paths accept directories, files, globs (`*`, `**`, `?`, character
classes and brace alternatives), and `!` exclusions. Pi's `+path` and `-path`
overrides select exact declared files; a skill directory can also be the target.
Skills must use `SKILL.md` directories. Prompts are Markdown files, collected
recursively from declared directories. Glob discovery skips dot-prefixed paths;
list those paths directly. Symlink resource roots are unsupported.

Agent declarations follow
[`@tintinweb/pi-subagents`' package format](https://github.com/tintinweb/pi-subagents/blob/main/docs/packages.md):
direct files or directories containing immediate `*.md` files, with exact `!`
exclusions and no globs. The provider's array shorthand and `pi-subagents` key
are accepted; `pi.subagents` takes precedence when both are supplied.

When a `pi` manifest exists, omitted or empty skill, prompt, and agent declarations
expose none of that type. Hangar does not scan conventional directories to add
undeclared resources. Without a Pi manifest, existing directory discovery stays
available. Rules bundles remain a separate Hangar feature: Pi has no package
rules field. Declared resources support the same selection, previews, listing,
updates, sync, and removal as other Hangar content.

This installs selected content files through Hangar's store and lockfile. For
whole packages with extensions, themes, runtime dependencies, or executable
workflows, use Pi's native `pi install`. Profiles can capture those package
registrations and restore them through `pi install`, retaining saved resource
filters. Direct content installs do not register whole packages or run dependency
installation. See
[Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md).

### Discover, select, and manage

Sources may contain `agents/*.md` or `agents/*.toml`, `prompts/*.md`,
`commands/*.md`, and `rules/` directories, including under `.pi`, `.codex`,
or `.cursor`. Agents and prompts are individual files directly inside those
directories. Each **outermost** `rules/` directory is one selectable bundle;
nested rules directories belong to that same bundle. Codex `.rules` policies
inside `rules/` are separate file entries and install directly into its native
rules directory; they are never compiled as Markdown. Bundle names use the
relative directory path with `/` replaced by `-` (for example `rules` or
`backend-rules`). A source rooted directly at a rules directory uses `rules`.
Existing skill discovery remains available alongside these entries.

```sh
hangar list ./my-content --json
hangar list ./my-content --type prompt
hangar install ./my-content --type prompt -a pi --all
hangar install ./my-content --type rule -a pi --all
hangar install owner/repo#agent:reviewer -a codex
hangar install ./my-content --type prompt -a codex --global --all
hangar info prompt:review
hangar update prompt:review
hangar remove prompt:review
```

The picker, preview, review, and installed list display content types. Use
`kind:name` (`skill`, `ref`, `agent`, `rule`, `prompt`) when names collide;
ambiguous installed bare names are rejected. `--type` filters installs and
source/installed listings. Git source `#kind:name` selectors also work; npm's
`#file` selector retains its reference-document meaning.

New types reuse `.agents/hangar.lock` and canonical `.agents/agents/`,
`.agents/rules/`, and `.agents/prompts/` stores. The lock records destinations
and target harnesses so updates and removals work after installation even if
a harness is not auto-detected. Local definitions and prompts are symlinked;
global files are copied. Compiled instructions are regular files. New content
uses the existing skill sanitization (invisible characters removed by default,
authored comments preserved), including `.mdc`, `.toml`, and `.rules` files.

### Choose the Pi rules destination

Use `--pi-rules-file` on `install` or `update` to choose where Pi loads a rules
bundle. The TUI has the same choice in **Install options**, shown again on the
review screen.

| Choice | Project location | Behavior |
| --- | --- | --- |
| `APPEND_SYSTEM.md` | `.pi/APPEND_SYSTEM.md` | Adds to the system prompt; the trusted project file takes precedence over the global append file. |
| `AGENTS.md` | `AGENTS.md` | Context instructions; global and ancestor context files can also apply. |
| `AGENTS.override.md` | `AGENTS.override.md` | Context instructions replacing `AGENTS.md`/`CLAUDE.md` in the same directory. |

With `--global`, all three filenames go under `~/.pi/agent/`. These choices
follow [Pi's instruction discovery](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/configuration.md).
New installations retain `APPEND_SYSTEM.md` as the default. Once installed,
the lockfile remembers the choice, so updates, lockfile sync, and profile
application retain it unless explicitly changed.

```sh
# Put a rules bundle in project context
hangar install ./my-content --type rule -a pi --all --pi-rules-file AGENTS.md

# Change that installed bundle's destination
hangar update rule:rules --pi-rules-file AGENTS.override.md

# Subsequent updates retain the choice
hangar update rule:rules
```

Changing the destination removes the previous managed Pi output after writing
the new one, retaining any destination still used by another installed harness.
An existing unmanaged destination is refused. Separate named bundles can use
different instruction files, for example one in `AGENTS.md` and another in
`APPEND_SYSTEM.md`; each destination can be owned by one installed bundle.
The option applies to complete bundles and does not interpret rule frontmatter.

### Compile rules explicitly

```sh
# Pi's loaded project instruction location
hangar compile-rules ./rules ./.pi/APPEND_SYSTEM.md

# Project context instructions (Pi or Codex)
hangar compile-rules ./rules ./AGENTS.md

# Pi context override in the same directory
hangar compile-rules ./rules ./AGENTS.override.md

# Alternate output, only useful with explicit harness configuration
hangar compile-rules ./rules ./AGENTS.append.md
```

`compile-rules <directory> <output-path>` recursively collects every `.md`
file beneath the directory passed in, including files inside nested `rules/`
directories. It sorts by relative path, includes each file once, preserves its
Markdown verbatim, and inserts a blank line between files. Non-Markdown files
are ignored. It writes one complete output, replacing the previous generated
content on repeat runs; it never appends to the old output or generates
separate outputs for nested rules directories.

The output filename must be `APPEND_SYSTEM.md`, `AGENTS.md`,
`AGENTS.override.md`, or `AGENTS.append.md`, and must be **outside** the source rules directory. Missing
sources, empty Markdown sets, and write failures return errors. Existing
unmanaged instruction files are refused. Explicit compilation records ownership
in `.agents/hangar.lock` **beside the output**; retain that lock to regenerate
it. Explicit compilations are regenerated with `compile-rules`, separately from
the install/update/remove lifecycle of discovered content.

Pi loads `.pi/APPEND_SYSTEM.md` locally and
`~/.pi/agent/APPEND_SYSTEM.md` globally. Its project file takes precedence over
the global file. A root `APPEND_SYSTEM.md` or `AGENTS.append.md` is not an
equivalent destination. Pi installations choose a native destination through
`--pi-rules-file`; Codex Markdown rules choose `AGENTS.md`.

### Prototype limitations

- One compiled rules bundle per destination file. Combine related rules under
  one outermost directory, or install separate named bundles to different Pi
  instruction files. Cursor keeps native `.mdc` rules in a
  bundle directory; plain `.md` files there are ignored by Cursor. Pi and Codex
  compilation includes `.md` files only. Codex native `.rules` policies retain
  their authored Starlark syntax, and project policies require a trusted
  `.codex` config layer. Hangar does not interpret or convert command policies.
- No merging into an unmanaged instruction file. Move existing instructions
  into the source rules directory or choose a separate managed workflow.
  Keep generated instruction files owned by Hangar; edits belong in the sources.
- Agent definitions are single native files. Supporting files referenced by
  agent configurations or prompts are not vendored. Duplicate names within the
  same content type require selecting a narrower source directory.
- Codex custom prompts are deprecated and global-only. Use skills for new
  reusable Codex workflows. Pi agents require the separately installed
  `@tintinweb/pi-subagents` provider. Include its native Pi package in a profile
  to restore it alongside definitions; direct agent installs do not install it.
  Hangar does not author workflows or manage the provider's own settings.
- Default harness locations only; custom `CODEX_HOME` and
  `PI_CODING_AGENT_DIR` locations are not handled. Reload Pi or restart the
  relevant harness session after changing resources; trust/configuration can
  control whether project resources load.
- Changing a pinned
  ref through `hangar pin <name> <ref>` is limited to existing skills/references;
  install a new source at `@ref` for agents, rules, or prompts. `doctor` retains
  its existing skills/references checks and does not diagnose new content stores
  or explicitly compiled outputs.
- JSON retains the existing `skills` array names for compatibility; entries
  carry their actual `kind`, including agents, rules, and prompts.

## Quick start

```sh
# Browse the curated catalog and pick interactively
hangar

# Install from a GitHub repo — opens the picker when there are many skills
hangar install anthropics/skills

# Install one skill directly from a subpath
hangar install anthropics/skills/skills/pdf

# Install everything, no prompts
hangar install anthropics/skills --all

# Install from npm (SKILL.md files become skills; README/docs become references)
hangar install npm:@your-scope/skills-pkg

# Keep things current
hangar update

# Check the install is healthy
hangar doctor
```

## Source specifiers

| Form                       | Example                                               |
| -------------------------- | ----------------------------------------------------- |
| GitHub repo                | `owner/repo`                                          |
| …with a subpath            | `owner/repo/path/to/skills`                           |
| …pinned to a ref           | `owner/repo@v1.2.0` or `owner/repo@release/1.x`       |
| …a single skill            | `owner/repo#skill-name`                               |
| GitHub URL                 | `https://github.com/owner/repo/tree/main/sub`         |
| GitLab repo / URL          | `https://gitlab.com/group/proj/-/tree/main/sub`       |
| Bitbucket repo / URL       | `https://bitbucket.org/owner/repo/src/main/sub`       |
| Forgejo / Gitea / Codeberg | `https://codeberg.org/owner/repo/src/branch/main/sub` |
| Tangled                    | `tangled:owner/repo`, `https://tangled.org/owner/repo` |
| npm package                | `npm:lodash`, `npm:@scope/pkg`                        |
| …a version / subpath / doc | `npm:pkg@1.2.0`, `npm:pkg/sub`, `npm:pkg#docs/api.md` |
| HTTP skill file            | `https://atproto.md/skill.md`                         |
| local path                 | `./path`, `/abs/path`, `~/path`, `file://…`           |

A pasted browser or clone URL from any supported host "just works"; the ref and
subpath are read from the provider's URL markers. A bare `owner/repo` always
means GitHub. An `http(s)://` URL whose host is not a recognized forge is
fetched directly as a single skill file. Refs are taken from the **last** `@`,
so they may contain `/`. Subpaths can never escape the repository root.

### Other Git hosts & private repos

GitHub auth comes from `$GH_TOKEN`/`$GITHUB_TOKEN` or the `gh` CLI. For the other
providers, set a per-provider token — `$GITLAB_TOKEN`, `$BITBUCKET_TOKEN`,
`$FORGEJO_TOKEN`/`$GITEA_TOKEN` — or store tokens (and register **self-hosted
hosts**) in `~/.config/hangar/config.toml`:

```toml
[forges]
gitlab_token    = "..."
bitbucket_token = "..."

# Register a self-hosted host so its URLs are recognized:
[forges.hosts."git.company.com"]
type  = "gitlab"            # gitlab | forgejo | gitea | bitbucket | tangled | generic
token = "${COMPANY_TOKEN}"  # ${ENV} references are expanded
```

Environment variables take precedence over config-file tokens. A project-local
`./hangar.toml` overrides the user file.

Tangled (`tangled.org`, or the `tangled.sh` alias) is public and needs no token.
The `tangled:owner/repo` shorthand is equivalent to a `tangled.org/owner/repo`
URL.

## The interactive picker

Run `hangar` to browse the curated catalog (grouped by category — Documents,
Design & Art, Web & Frontend, Development & Agents, Writing & Comms — plus
free-form source entry), or `hangar install <source>` on a multi-skill source to
open the picker directly:

- **Tree view** — folders carry tri-state checkboxes (`[x]` / `[~]` / `[ ]`);
  toggling a folder cascades to the skills beneath it.
- **`t`** — switch between the tree and a flat list. Your selection is preserved.
- **`/`** — incrementally filter by name, description, or path (matches keep
  their ancestors and auto-expand in the tree).
- **Right pane** — a live, glamour-rendered preview of the highlighted
  `SKILL.md`. Press **`tab`** to focus it and scroll (`↑/↓`, `pgup/pgdn`);
  `esc`/`tab` returns to the list.
- **`space`** toggle · **`→`/`←`** expand/collapse folders · **`a`/`n`** all/none
  · **`⏎`** continue · **`esc`** back · **`q`** quit.

Then choose target agents (detected ones are pre-selected), set the scope and
sanitization on the options screen, and review. The review screen lets you jump
back to any step (`s` skills · `a` agents · `o` options), tweak scope/sanitize
inline (`g`/`i`/`c`), and page the full selected-skill list (`f`).

## On-disk layout

Hangar uses the shared `.agents/` convention so installs interoperate with the
wider ecosystem:

```
.agents/
  skills/<name>/SKILL.md      canonical store (real files)
  references/<name>/REFERENCE.md
  hangar.lock                 TOML lockfile
.claude/skills/<name> -> ../../.agents/skills/<name>   (per-agent symlink, local)
AGENTS.md                     managed <references> block (also CLAUDE.md, …)
```

Local (project) installs symlink each agent's skills dir to the canonical store;
global installs (`-g`) copy instead. References (npm READMEs/docs) are written to
`.agents/references/` and linked from a managed block in your instructions file.

## Security

Third-party skills are sanitized on install:

- **Invisible characters** (zero-width, bidi overrides, the Unicode Tag block)
  are stripped from all content.
- **Markdown comments** (`<!-- … -->` and `[//]: #`) are additionally stripped
  from references — outside fenced code blocks.
- `update` flags **tag rewrites** (a pinned tag whose commit moved) as a
  high-severity finding.
- Headless installs print an **audit envelope** (a JSON record of every change,
  wrapped in a "treat as untrusted content" preamble) when stdout is not a TTY —
  so an agent invoking Hangar reviews changes before trusting them.

Disable passes with `--no-strip`, `--no-strip-comments`, `--no-strip-invisible`;
control the envelope with `--audit` / `--no-audit`.

## Commands

| Command                                | Purpose                                                 |
| -------------------------------------- | ------------------------------------------------------- |
| `hangar`                               | Browse the catalog and install interactively            |
| `hangar install <source>`              | Install skills (interactive or headless)                |
| `hangar install`                       | Reinstall everything from the lockfile                  |
| `hangar list`                          | Manage installed skills (interactive on a terminal)     |
| `hangar list <source>`                 | List the skills/references a source contains            |
| `hangar info <skill\|source>`          | Render a skill's SKILL.md                               |
| `hangar update [skill]`                | Re-resolve and update installed skills                  |
| `hangar pin <skill> [ref]`             | Pin a skill (optionally reinstall at a ref)             |
| `hangar unpin <skill>`                 | Clear a skill's pinned flag                             |
| `hangar remove <skill>`                | Remove a skill/reference everywhere                     |
| `hangar nuke [-y]`                     | Remove every installed skill/reference (respects flags) |
| `hangar profile save\|apply\|list\|show\|rm` | Save, inspect, and restore content and Pi package sets |
| `hangar doctor [--fix]`                | Diagnose and repair drift                               |

Common flags: `-a/--agent` (repeatable target), `-g/--global`, `-A/--all`,
`-y/--yes`, `--no-tty`, `--json`, `-v/--verbose`, `--cwd`.

### Managing installed skills

`hangar list` (no source) opens an interactive manager on a terminal: it flags
outdated skills (`⬆ <version>`), and lets you preview a pending update as a
unified **diff** (`⏎`), **update** one (`u`) or all (`U`), **pin**/unpin (`p`),
and **remove** (`x`). Headless or `--json`, it prints the same status.

### Profiles

Profiles capture installed skills, references, agent definitions, rules bundles,
prompts, and native Pi package registrations. Saved entries retain their content
type, source path, pin, harness targets, package-manifest origin, and Pi rules
filename. Applying a profile recomputes destinations for the selected local or
global scope and uses the saved harnesses by default.

```sh
hangar profile save backend     # snapshot Hangar content and local Pi packages
hangar profile show backend     # typed entries, harnesses, rules destinations
hangar profile show backend --json
cd ../other-project
hangar profile apply backend    # install the same set here

# Work with just one type
hangar profile save reviewers --type agent
hangar profile apply backend --type prompt -a cursor
hangar profile apply backend --type rule --pi-rules-file APPEND_SYSTEM.md
hangar profile apply backend --type package

# Edit the saved set without removing installed content
hangar profile remove backend agent:reviewer
hangar profile remove backend package:npm:@tintinweb/pi-subagents

# Save from or apply to the global scope
hangar profile save personal --global
hangar profile apply personal --global
```

`save`, `show`, and `apply` accept `--type skill|ref|agent|rule|prompt|package`.
`apply -a <harness>` overrides saved content targets; packages require Pi among
the targets when an override is supplied. Existing skill-only profiles continue
to load. Older entries without recorded harnesses use normal auto-detection.
Applying adds or updates the selected entries and leaves other installed content
in place. `remove <profile>` deletes the profile; adding a typed entry removes
only that item, deleting the profile if it becomes empty. Shared names require
`kind:name`.

Whole Pi packages are read from `.pi/settings.json` or, with `--global`,
`~/.pi/agent/settings.json`. Only that scope's `packages` array is captured;
other Pi settings and the provider's own configuration are not included. Local
package paths are saved as absolute paths, so those directories must be accessible
when restoring. Object-form filters, empty resource arrays, and `autoload` are
preserved in the profile's `[[packages]]` entries.

Package restoration requires `pi` on PATH and calls `pi install <source> --local --approve`
for project scope or `pi install <source>` globally. Pi handles downloading,
dependencies, and package identity; Hangar restores saved resource filters after
installation while retaining other settings and package declarations. Errors stop
the apply; earlier successful entries remain installed. The project install uses
Pi's process-only approval to modify local package configuration, without saving
a trust decision. Subsequent Pi sessions apply their normal project trust settings.
See [Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md).

On a terminal, `hangar profile list` opens the profile browser: inspect typed
entries and package options, remove individual items with `x`, toggle the apply
scope with `g`, or apply the profile with `a`. Standalone `compile-rules` outputs
are not profile entries; install a discovered rules bundle to include it.

### Headless & scripting

Hangar runs non-interactively when stdout isn't a terminal, in CI, with
`--no-tty`, or with `--all`/`-y`. Add `--json` to `list`, `install`, `update`,
and `doctor` for machine-readable output:

```sh
hangar list anthropics/skills --json
hangar doctor --json   # exits non-zero if unhealthy
```

## Preferences

User preferences persist to `~/.config/hangar/config.toml` (XDG-aware): the
picker view (tree/list), default agents, scope, and sanitize toggles. CLI flags
always override the stored value for a run.

## npm & private registries

npm packages are fetched from the registry, integrity-verified
(`dist.integrity`/`shasum`), and crawled for skills and docs. Hangar honors
`.npmrc`: the default `registry`, per-scope `@scope:registry`, and
`//host/:_authToken` (with `${ENV}` expansion) for private and scoped registries.

## Development

```sh
go build ./...
go test ./...
```

The codebase is layered with a strict dependency direction
(`cmd → {tui, present, engine}`; engine packages never import the TUI). The TUI
talks to the rest of Hangar only through `tui.EngineAPI`, so screens are tested
against a fake engine.

## Contributing

Contributions are welcome. A couple of conventions keep the project releasable on
autopilot — see [Development](#development) for build/test commands and the
layering rules.

### Commit messages — Conventional Commits

Releases are cut automatically from commit history, so commits (and **PR titles**,
since PRs are squash-merged) follow
[Conventional Commits](https://www.conventionalcommits.org):

| Prefix                                     | Example                      | Effect on the next release |
| ------------------------------------------ | ---------------------------- | -------------------------- |
| `fix:`                                     | `fix: handle empty lockfile` | Patch (`0.1.0` → `0.1.1`)  |
| `feat:`                                    | `feat: add scoop support`    | Minor (`0.1.0` → `0.2.0`)  |
| `feat!:` / `BREAKING CHANGE:` footer       | `feat!: drop legacy layout`  | Major\*                    |
| `docs:`, `refactor:`, `chore:`, `test:`, … | `chore: bump deps`           | No release on its own      |

\* While Hangar is pre-1.0, breaking changes bump the **minor** version, not the
major. Force an exact version any time with a `Release-As: X.Y.Z` commit footer.

### How a release happens

Releases are fully automated via
[release-please](https://github.com/googleapis/release-please) and
[GoReleaser](https://goreleaser.com):

1. Conventional commits land on `main`.
2. release-please opens and keeps updating a **release PR** that bumps the version
   and writes `CHANGELOG.md`.
3. A maintainer merges the release PR — that tags the version, publishes the
   GitHub Release with binaries for Linux/macOS/Windows, and updates the Homebrew
   tap and Scoop bucket.

## License

MIT
