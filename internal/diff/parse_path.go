package diff

import (
	"slices"
	"strings"
)

// mnemonicPrefixes are the single-character prefixes git uses with diff.mnemonicPrefix=true.
// "1/" and "2/" are used by jj diff --git.
var mnemonicPrefixes = []string{"i/", "c/", "w/", "o/", "1/", "2/"}

// normalizeDiffGitLine rewrites the "diff --git X Y" line's prefixes.
func normalizeDiffGitLine(line string) string {
	// line format: "diff --git <src> <dst>\n"
	const prefix = "diff --git "
	rest := strings.TrimPrefix(line, prefix)
	trail := ""
	if strings.HasSuffix(rest, "\n") {
		trail = "\n"
		rest = rest[:len(rest)-1]
	}

	// Handle C-quoted path pair: "a/..." "b/..."
	if strings.HasPrefix(rest, `"`) {
		src, dst, ok := splitQuotedGitDiffPair(rest)
		if ok {
			src = unquoteGitPath(src)
			dst = unquoteGitPath(dst)
			newSrc := normalizePathPrefix(src, "a/")
			newDst := normalizePathPrefix(dst, "b/")
			return prefix + newSrc + " " + newDst + trail
		}
	}

	// For unquoted paths, try a midpoint split when the token count is even —
	// this handles space-containing filenames that appear symmetrically.
	tokens := strings.Split(rest, " ")
	if len(tokens) >= 2 && len(tokens)%2 == 0 {
		half := len(tokens) / 2
		src := strings.Join(tokens[:half], " ")
		dst := strings.Join(tokens[half:], " ")
		if hasKnownPrefix(src) && hasKnownPrefix(dst) {
			newSrc := normalizePathPrefix(src, "a/")
			newDst := normalizePathPrefix(dst, "b/")
			return prefix + newSrc + " " + newDst + trail
		}
	}

	// Fallback: last-space split (safe for simple paths without spaces).
	spaceIdx := strings.LastIndex(rest, " ")
	if spaceIdx < 0 {
		return line
	}
	src := rest[:spaceIdx]
	dst := rest[spaceIdx+1:]
	newSrc := normalizePathPrefix(src, "a/")
	newDst := normalizePathPrefix(dst, "b/")
	return prefix + newSrc + " " + newDst + trail
}

// splitQuotedGitDiffPair splits `"<src>" "<dst>"` into the two quoted tokens.
func splitQuotedGitDiffPair(s string) (src, dst string, ok bool) {
	// Find the closing quote of the first token.
	i := 1 // skip opening "
	for i < len(s) {
		if s[i] == '\\' {
			i += 2
			continue
		}
		if s[i] == '"' {
			src = s[:i+1] // includes surrounding quotes
			rest := strings.TrimPrefix(s[i+1:], " ")
			if strings.HasPrefix(rest, `"`) && strings.HasSuffix(rest, `"`) {
				return src, rest, true
			}
			return "", "", false
		}
		i++
	}
	return "", "", false
}

// unquoteGitPath removes surrounding double-quotes and C-style backslash escapes.
func unquoteGitPath(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	s = s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// hasKnownPrefix returns true if the path starts with a/, b/, or a mnemonic prefix.
func hasKnownPrefix(path string) bool {
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") {
		return true
	}
	return slices.ContainsFunc(mnemonicPrefixes, func(p string) bool {
		return strings.HasPrefix(path, p)
	})
}

// normalizePathPrefix strips mnemonic or absent prefixes and adds canonical.
// Leaves a/, b/, /dev/null and absolute paths untouched.
func normalizePathPrefix(path, canonical string) string {
	// Strip trailing newline for processing; restore at end.
	trail := ""
	if strings.HasSuffix(path, "\n") {
		trail = "\n"
		path = path[:len(path)-1]
	}
	// Already canonical or special path.
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") || strings.HasPrefix(path, "/") {
		return path + trail
	}
	// Strip mnemonic prefix and replace with canonical.
	for _, p := range mnemonicPrefixes {
		if rest, ok := strings.CutPrefix(path, p); ok {
			return canonical + rest + trail
		}
	}
	// No prefix — add canonical.
	return canonical + path + trail
}
