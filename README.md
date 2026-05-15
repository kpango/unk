# unk

**A terminal-first diff viewer built for reviewing coding-agent changesets.**

`unk` brings a desktop-quality code review experience to your terminal. It parses unified diffs from Git (or Jujutsu), renders them in a polished TUI with syntax highlighting, and surfaces AI agent annotations inline — so you can understand *why* a change was made, not just *what* changed.

---

## Features

- **Side-by-side and stacked layouts** — auto-switches based on terminal width, or set manually
- **Syntax highlighting** — language-aware coloring for additions, deletions, and context lines
- **Agent annotation support** — display AI-generated rationale and per-hunk notes from a JSON sidecar
- **Three keymap styles** — Helix (default), Vim, and Emacs
- **Eight built-in themes** — dark, light, and Catppuccin/Solarized variants; define your own
- **Watch mode** — live reload when the underlying diff changes (`--watch`)
- **Git pager** — drop-in replacement for `core.pager` with diff detection and plain-text fallback
- **Git difftool** — integrates as `git difftool` for file-pair comparisons
- **Jujutsu support** — works with `jj` repositories out of the box
- **Layered config** — global and per-repo YAML/TOML with CLI flag overrides
- **Command mode** — Vim/Helix-style `:command` prompt for in-session control

---

## Installation

### Go (recommended)

```sh
go install github.com/kpango/unk/cmd/unk@latest
```

### Build from source

```sh
git clone https://github.com/kpango/unk.git
cd unk
make build          # produces bin/unk
```

> **Requirements:** Go 1.26+ and a terminal with at least 256-color support (true color recommended).

---

## Quick start

```sh
# Review working-tree changes
unk diff

# Review staged changes
unk diff --staged

# Review the last commit
unk show

# Review a specific commit or range
unk show abc1234
unk diff HEAD~3..HEAD

# Review a patch file
unk patch my.patch

# Pipe a patch from stdin
git diff | unk patch

# Review a stash entry
unk stash show
unk stash show stash@{2}
```

---

## Commands

### `unk diff [target] [-- pathspec...]`

Review working-tree changes or compare two refs.

```sh
unk diff                            # unstaged working-tree
unk diff --staged                   # staged changes
unk diff HEAD~1                     # changes since one commit ago
unk diff main..feature              # branch comparison
unk diff HEAD -- src/               # limit to a subdirectory
unk diff old.go new.go              # two arbitrary files
```

| Flag | Default | Description |
|------|---------|-------------|
| `--staged` / `--cached` | false | Show staged (index) changes |
| `--watch` | false | Auto-reload when the diff changes |
| `--exclude-untracked` | false | Hide untracked files from the sidebar |

### `unk show [ref] [-- pathspec...]`

Review a commit, tag, or any ref.

```sh
unk show                  # HEAD
unk show HEAD~2           # two commits back
unk show v1.4.0           # a tag
unk show abc1234 -- api/  # filtered to a path
```

### `unk stash show [ref]`

Review a stash entry.

```sh
unk stash show            # stash@{0}
unk stash show stash@{1}  # a specific stash
```

### `unk patch [file]`

Review a unified diff from a file or stdin.

```sh
unk patch changes.patch
git format-patch -1 HEAD | unk patch
curl https://example.com/some.patch | unk patch
```

### `unk pager`

A drop-in Git pager. Detects unified diff output and renders it with the full TUI; falls back to plain text for non-diff content.

```sh
# Set globally
git config --global core.pager 'unk pager'

# Or per-repo
git config core.pager 'unk pager'

# Pipe manually
git log -p | unk pager
```

> When stdout is not a TTY (e.g., piped to `grep`) or `$TERM=dumb`, `unk pager` passes through transparently.

### `unk difftool <left> <right> [path]`

Review Git difftool file pairs.

```sh
# Register as the default difftool
git config --global diff.tool unk
git config --global difftool.unk.cmd 'unk difftool $LOCAL $REMOTE $MERGED'

# Then run normally
git difftool HEAD~1
```

---

## Common flags

These flags are accepted by all commands:

| Flag | Description |
|------|-------------|
| `--mode auto\|split\|stack` | Layout mode (default: `auto`) |
| `--theme <name>` | Named color theme |
| `--keymap helix\|vim\|emacs` | Key binding style (default: `helix`) |
| `--agent-context <file>` | JSON sidecar with AI agent annotations |
| `--pager` | Enable pager-style chrome |
| `--watch` | Auto-reload on change |
| `--line-numbers` / `--no-line-numbers` | Show/hide line numbers |
| `--wrap` / `--no-wrap` | Wrap / truncate long lines |
| `--unk-headers` / `--no-unk-headers` | Show/hide unk metadata rows |
| `--agent-notes` / `--no-agent-notes` | Show/hide agent annotation notes |

---

## Keyboard reference

Press `?` inside the TUI to open the in-session help overlay.

### Navigation

| Key (Helix) | Key (Vim) | Key (Emacs) | Action |
|-------------|-----------|-------------|--------|
| `]` | `]` | `]` | Next hunk |
| `[` | `[` | `[` | Previous hunk |
| `)` | `)` | `)` | Next file |
| `(` | `(` | `(` | Previous file |
| `}` | `}` | `}` | Next annotated hunk |
| `{` | `{` | `{` | Previous annotated hunk |

### Scrolling

| Key (Helix) | Key (Vim) | Key (Emacs) | Action |
|-------------|-----------|-------------|--------|
| `↑` / `k` | `↑` / `k` | `↑` / `C-p` | Scroll up |
| `↓` / `j` | `↓` / `j` | `↓` / `C-n` | Scroll down |
| `C-u` | `u` | `C-u` | Half page up |
| `C-d` | `d` | `C-d` | Half page down |
| `pgup` / `C-b` | `b` / `pgup` | `M-v` / `pgup` | Page up |
| `pgdn` / `C-f` / `space` | `f` / `space` | `C-v` / `pgdn` | Page down |
| `home` / `g` | `home` / `g` | `home` | Top |
| `end` / `G` | `end` / `G` | `end` | Bottom |
| `←` | `←` | `←` / `C-b` | Scroll left |
| `→` | `→` | `→` / `C-f` | Scroll right |
| `S-←` | `S-←` | `S-←` | Scroll left ×8 |
| `S-→` | `S-→` | `S-→` | Scroll right ×8 |

### Layout and view

| Key | Action |
|-----|--------|
| `1` | Side-by-side (split) layout |
| `2` | Vertical (stack) layout |
| `0` | Auto layout |
| `s` | Toggle file sidebar |
| `l` | Toggle line numbers |
| `w` | Toggle line wrapping |
| `m` | Toggle unk metadata headers |
| `a` | Toggle agent annotation notes |
| `t` | Cycle to next theme |
| `tab` | Move focus between sidebar and diff pane |

### Actions

| Key | Action |
|-----|--------|
| `/` | Filter sidebar by filename |
| `\` | Grep / search diff content |
| `n` | Next search match |
| `N` | Previous search match |
| `o` | Open current file in `$EDITOR` |
| `y` | Yank current hunk patch to clipboard |
| `r` | Refresh / reload diff |
| `:` | Enter command mode |
| `?` | Toggle help overlay |
| `q` / `esc` | Quit |
| `C-z` | Suspend |

---

## Command mode

Press `:` to open the command prompt. Tab-completion is supported.

| Command | Alias | Description |
|---------|-------|-------------|
| `quit` | `q`, `q!` | Exit unk |
| `help` | `h` | Show the help overlay |
| `split` | `sp` | Switch to side-by-side layout |
| `stack` | `st` | Switch to vertical layout |
| `auto` | | Switch to auto layout |
| `wrap` | | Toggle line wrapping |
| `number` | `numbers`, `nu` | Toggle line numbers |
| `sidebar` | `sb` | Toggle the file sidebar |
| `headers` | `hd` | Toggle unk metadata header rows |
| `notes` | | Toggle agent annotation notes |
| `theme [name]` | `t [name]` | Cycle theme or jump to named theme |
| `keymap [style]` | `km [style]` | Change keymap style (`helix`, `vim`, `emacs`) |
| `filter [query]` | `f [query]` | Filter sidebar by filename |
| `search [query]` | `grep`, `g` | Search diff content |
| `reload` | `e` | Reload current diff input |
| `first` | | Jump to the first file |
| `last` | | Jump to the last file |

---

## Themes

Eight themes are bundled. Press `t` to cycle through them or set one with `--theme <name>`:

| Name | Style |
|------|-------|
| `graphite` | Dark (default) |
| `midnight` | Dark |
| `paper` | Light |
| `ember` | Dark warm |
| `catppuccin-mocha` | Dark |
| `catppuccin-latte` | Light |
| `solarized-dark` | Dark |
| `solarized-light` | Light |

---

## Configuration

`unk` reads configuration from layered sources, applied in order from lowest to highest priority:

```
built-in defaults
  → global config.toml
  → global config.yaml
  → repo config.toml   (.unk/config.toml)
  → repo config.yaml   (.unk/config.yaml)
  → CLI flags
```

YAML and TOML are both supported at each level. When both exist in the same directory, YAML takes precedence.

### Config file locations

| Scope | Path |
|-------|------|
| Global (preferred) | `$XDG_CONFIG_HOME/unk/config.yaml` |
| Global (fallback) | `~/.config/unk/config.yaml` |
| Per-repository | `<repo-root>/.unk/config.yaml` |

TOML equivalents: `config.toml` in the same directories.

### Full config.yaml reference

```yaml
# Layout mode: "auto" (default), "split", "stack"
mode: auto

# VCS backend: "git" (auto-detected), "jj"
vcs: git

# Color theme (see Themes section)
theme: graphite

# Key-binding style: "helix" (default), "vim", "emacs"
keymap: helix

# Display options
line_numbers: true
wrap_lines: false
unk_headers: true
agent_notes: false
exclude_untracked: false

# Per-command overrides (keys: vcs, show, stash-show, diff, patch, difftool, pager)
commands:
  vcs:
    mode: split
  show:
    theme: paper
    line_numbers: false
  pager:
    wrap_lines: true

# Custom key bindings (override individual actions on top of the base keymap)
keybindings:
  next_unk: ["]", "ctrl+n"]
  prev_unk: ["[", "ctrl+p"]
  scroll_down: ["j", "down"]
  scroll_up: ["k", "up"]
  quit: ["q", "ctrl+c"]

# Custom color themes
themes:
  my-dark:
    panel_alt: "#1d2126"
    panel: "#171a1d"
    border: "#343c45"
    accent: "#d5e0ea"
    text: "#f2f4f6"
    muted: "#9aa4af"
    accent_muted: "#414a54"
    added_bg: "#1f3025"
    removed_bg: "#372526"
    context_bg: "#181c20"
    added_content_bg: "#24362a"
    removed_content_bg: "#432b2d"
    added_sign_color: "#88d39b"
    removed_sign_color: "#f0a0a0"
    unk_header_fg: "#9aa4af"
    line_number_bg: "#14181b"
    line_number_fg: "#798592"
    badge_added: "#88d39b"
    badge_removed: "#f0a0a0"
    badge_neutral: "#a9b4bf"
    file_new: "#88d39b"
    file_deleted: "#f0a0a0"
    file_renamed: "#e6cf98"
    file_modified: "#c49bff"
    file_untracked: "#7fd1ff"
    note_border: "#c6a0ff"
    note_background: "#241c31"
    note_title_background: "#322446"
    note_title_text: "#f5edff"
```

A fully-annotated reference file is available at [`example/config.yaml`](example/config.yaml).

### TOML config

All options from `config.yaml` are available in TOML format as `config.toml`. TOML supports per-command sections:

```toml
mode     = "auto"
theme    = "graphite"
keymap   = "helix"
vcs      = "git"

[vcs]
mode = "split"

[show]
theme = "paper"

[themes.my-dark]
panel  = "#171a1d"
accent = "#d5e0ea"
```

---

## Agent context

`unk` can display AI-generated annotations alongside the diff. Pass a JSON sidecar with `--agent-context <file>`:

```sh
unk diff --agent-context review.json
unk show HEAD --agent-context notes.json
```

### Agent context JSON schema

```json
{
  "version": 1,
  "summary": "Optional overall summary of the changeset",
  "files": [
    {
      "path": "internal/loader/loader.go",
      "summary": "Optional per-file summary",
      "annotations": [
        {
          "id": "optional-unique-id",
          "summary": "Why this change was made",
          "rationale": "Longer explanation (optional)",
          "tags": ["performance", "refactor"],
          "confidence": "high",
          "newRange": [42, 58],
          "oldRange": [40, 55],
          "source": "claude-opus-4",
          "author": "agent",
          "createdAt": "2026-01-15T10:30:00Z"
        }
      ]
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `version` | `int` | Schema version (currently `1`) |
| `summary` | `string?` | Top-level changeset summary |
| `files[].path` | `string` | File path relative to repo root |
| `files[].summary` | `string?` | Per-file summary |
| `annotations[].summary` | `string` | Short annotation text (required) |
| `annotations[].rationale` | `string?` | Extended explanation |
| `annotations[].tags` | `[]string?` | Categorization tags |
| `annotations[].confidence` | `"low"\|"medium"\|"high"?` | Annotation confidence level |
| `annotations[].newRange` | `[start, end]?` | Line range in the new file (1-based) |
| `annotations[].oldRange` | `[start, end]?` | Line range in the old file (1-based) |

Annotations are rendered inline inside the diff pane. Toggle their visibility with `a` or `--agent-notes`.

---

## Keybinding overrides

Individual bindings can be remapped without changing the base keymap style. In your config:

```yaml
keymap: helix   # base style

keybindings:
  # Override individual actions
  next_unk: ["]", "ctrl+n"]
  prev_unk: ["[", "ctrl+p"]
  scroll_down: ["j", "down", "ctrl+j"]
  scroll_up: ["k", "up", "ctrl+k"]
  quit: ["q", "ctrl+c"]
```

**All available action names:**

| Category | Actions |
|----------|---------|
| Navigation | `next_unk`, `prev_unk`, `next_annotated_unk`, `prev_annotated_unk`, `next_file`, `prev_file` |
| Scrolling | `scroll_up`, `scroll_down`, `scroll_up_half`, `scroll_down_half`, `scroll_page_up`, `scroll_page_down`, `scroll_top`, `scroll_bottom`, `scroll_left`, `scroll_right`, `scroll_left_fast`, `scroll_right_fast` |
| Layout | `layout_split`, `layout_stack`, `layout_auto` |
| Toggles | `toggle_sidebar`, `toggle_line_numbers`, `toggle_wrap`, `toggle_unk_headers`, `toggle_agent_notes`, `cycle_theme` |
| Actions | `refresh`, `open_in_editor`, `yank_unk`, `focus_filter`, `focus_search`, `search_next`, `search_prev`, `command_mode`, `toggle_focus`, `help`, `quit`, `suspend` |
| Menu | `menu_open`, `menu_close`, `menu_left`, `menu_right`, `menu_up`, `menu_down`, `menu_confirm` |

**Key sequence syntax:**

```
Single chars:   "a"  "z"  "0"  "]"  "/"
Special keys:   "up"  "down"  "left"  "right"  "enter"  "tab"  "esc"
                "backspace"  "delete"  "home"  "end"  "pgup"  "pgdown"
                "f1"–"f12"  "space"
Modifiers:      "ctrl+c"  "alt+f"  "shift+left"
```

---

## Git integration

### As `core.pager`

```sh
git config --global core.pager 'unk pager'
```

`unk pager` renders diff output with the full TUI. When git prints non-diff text (e.g., `git log` without `-p`), it falls back to plain-text display automatically.

To configure pager-specific display options, add a `[pager]` (TOML) or `commands.pager` (YAML) section to your config:

```yaml
commands:
  pager:
    wrap_lines: true
    theme: midnight
```

### As `difftool`

```sh
git config --global diff.tool unk
git config --global difftool.unk.cmd 'unk difftool $LOCAL $REMOTE $MERGED'
git config --global difftool.prompt false

# Use it
git difftool HEAD~1
```

---

## Jujutsu (jj) support

`unk` auto-detects Jujutsu repositories (`.jj` directory) and uses `jj` as the VCS backend automatically. Override with `--vcs git` or set `vcs: git` in your config if needed.

```sh
# Works just like with git
unk diff
unk show @~1
```

---

## Development

### Prerequisites

- Go 1.26+

### Getting started

```sh
git clone https://github.com/kpango/unk.git
cd unk
make deps       # tidy and verify modules
make build      # compile bin/unk
```

### Development workflow

```sh
make format     # format all Go files with strictgoimports + crlfmt + golangci-lint fmt
make lint       # run go vet + golangci-lint
make test       # run the full test suite
make coverage   # generate HTML coverage report in test-results/
```

### Individual tasks

```sh
make format/go           # format non-test source files
make format/go/diff      # format only files changed since HEAD
make go/lint             # golangci-lint with auto-fix
make go/lint/nofix       # golangci-lint without auto-fix (CI mode)
make vet                 # go vet
make test/internal       # tests for internal/ only
make test/tparse         # tests with tparse summary table
make test/gotestfmt      # tests with gotestfmt output
make bench               # run all benchmarks
make bench/internal      # benchmark internal/ packages only
make tools/install       # install all dev tools
```

See `make help` for the full list of targets.

### Tool installation

```sh
make tools/install
```

This installs: `golangci-lint`, `goimports`, `strictgoimports`, `gofumpt`, `golines`, `crlfmt`, `tparse`, `gotestfmt`, `gopls`, `staticcheck`, `benchstat`.

### Linting and formatting

The project uses [golangci-lint](https://golangci-lint.run/) v2 with a curated set of linters defined in [`.golangci.json`](.golangci.json). Formatters include `gofumpt`, `goimports` (via `strictgoimports`), and `golines` (max line length: 200).

```sh
make format     # auto-format all files
make lint       # lint and report issues
```

### Project structure

```
unk/
├── cmd/unk/           # main entry point and CLI command definitions
├── internal/
│   ├── agent/         # agent annotation loading
│   ├── config/        # layered config resolution (TOML + YAML)
│   ├── diff/          # unified diff parser
│   ├── highlight/     # syntax highlighting (chroma)
│   ├── loader/        # diff loading pipeline (VCS → normalize → language)
│   ├── pager/         # plain-text pager and diff detection
│   ├── runner/        # top-level session runner
│   ├── tui/           # Bubbletea TUI (model, views, keys, styles, render)
│   ├── types/         # core shared data model
│   └── vcs/           # VCS backends (git, jj)
├── example/           # config examples
├── Makefile           # development task runner
└── Makefile.d/        # modular Makefile targets
```

---

## Contributing

Contributions are welcome. Please open an issue to discuss substantial changes before submitting a pull request.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-change`)
3. Run `make format lint test` before committing
4. Submit a pull request

---

## License

See [LICENSE](LICENSE) for details.
