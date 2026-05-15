package loader

import (
	"path/filepath"
	"strings"
)

// extensionLanguages maps common file extensions to language identifiers.
var extensionLanguages = map[string]string{
	".go":         "go",
	".ts":         "typescript",
	".tsx":        "typescriptreact",
	".js":         "javascript",
	".jsx":        "javascriptreact",
	".mjs":        "javascript",
	".cjs":        "javascript",
	".py":         "python",
	".pyi":        "python",
	".rb":         "ruby",
	".rs":         "rust",
	".java":       "java",
	".c":          "c",
	".h":          "c",
	".cpp":        "cpp",
	".cc":         "cpp",
	".cxx":        "cpp",
	".hpp":        "cpp",
	".hxx":        "cpp",
	".cs":         "csharp",
	".swift":      "swift",
	".kt":         "kotlin",
	".kts":        "kotlin",
	".sh":         "shellscript",
	".bash":       "shellscript",
	".zsh":        "shellscript",
	".fish":       "shellscript",
	".ps1":        "powershell",
	".psm1":       "powershell",
	".json":       "json",
	".jsonc":      "json",
	".yaml":       "yaml",
	".yml":        "yaml",
	".toml":       "toml",
	".md":         "markdown",
	".mdx":        "markdown",
	".html":       "html",
	".htm":        "html",
	".css":        "css",
	".scss":       "scss",
	".sass":       "sass",
	".less":       "less",
	".sql":        "sql",
	".tf":         "hcl",
	".hcl":        "hcl",
	".nix":        "nix",
	".lua":        "lua",
	".r":          "r",
	".R":          "r",
	".php":        "php",
	".php3":       "php",
	".php4":       "php",
	".php5":       "php",
	".phtml":      "php",
	".pl":         "perl",
	".pm":         "perl",
	".clj":        "clojure",
	".cljs":       "clojure",
	".cljc":       "clojure",
	".scala":      "scala",
	".sc":         "scala",
	".gradle":     "groovy",
	".groovy":     "groovy",
	".vue":        "vue",
	".svelte":     "svelte",
	".elm":        "elm",
	".hs":         "haskell",
	".lhs":        "haskell",
	".ml":         "ocaml",
	".mli":        "ocaml",
	".dart":       "dart",
	".jl":         "julia",
	".ex":         "elixir",
	".exs":        "elixir",
	".erl":        "erlang",
	".hrl":        "erlang",
	".tex":        "latex",
	".xml":        "xml",
	".xsl":        "xml",
	".xslt":       "xml",
	".svg":        "xml",
	".graphql":    "graphql",
	".gql":        "graphql",
	".proto":      "protobuf",
	".ini":        "ini",
	".cfg":        "ini",
	".properties": "properties",
	".env":        "shellscript",
	".lock":       "toml",
	".mod":        "go",
	".sum":        "go",
	".zig":        "zig",
	".cr":         "crystal",
	".nim":        "nim",
	".v":          "v",
	".dockerfile": "dockerfile",
}

// basenameLanguages maps exact filenames (no extension) to language identifiers.
var basenameLanguages = map[string]string{
	"Dockerfile":  "dockerfile",
	"Makefile":    "makefile",
	"makefile":    "makefile",
	"GNUmakefile": "makefile",
	"Jenkinsfile": "groovy",
	"Gemfile":     "ruby",
	"Rakefile":    "ruby",
	"Vagrantfile": "ruby",
	"CMakeLists":  "cmake",
}

// detectLanguage returns the language ID for a file path, or nil if unknown.
func detectLanguage(path string) *string {
	base := filepath.Base(path)
	// Check exact basename first (e.g. Dockerfile, Makefile).
	if lang, ok := basenameLanguages[base]; ok {
		return &lang
	}
	// Strip the extension and check again (handles "Dockerfile.dev", etc.).
	if ext := filepath.Ext(base); ext != "" {
		noExt := strings.TrimSuffix(base, ext)
		if lang, ok := basenameLanguages[noExt]; ok {
			return &lang
		}
		// Handle compound dotfile names like ".env.local", ".env.production":
		// if the leftmost component after the leading dot is a known dotfile type, use it.
		if strings.HasPrefix(noExt, ".env") {
			lang := "shellscript"
			return &lang
		}
	}
	// Dotfile with no extension (e.g. ".env"): check the whole basename.
	if strings.HasPrefix(base, ".env") && filepath.Ext(base) == "" {
		lang := "shellscript"
		return &lang
	}
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := extensionLanguages[ext]; ok {
		return &lang
	}
	return nil
}
