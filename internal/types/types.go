// Package types defines the core data model shared across all unk subsystems.
package types

// LayoutMode controls the diff pane arrangement.
type LayoutMode string

const (
	LayoutModeAuto  LayoutMode = "auto"
	LayoutModeSplit LayoutMode = "split"
	LayoutModeStack LayoutMode = "stack"
)

// VCSMode selects the version control backend.
type VCSMode string

const (
	VCSModeGit VCSMode = "git"
	VCSModeJJ  VCSMode = "jj"
)

// TerminalThemeMode is the terminal background luminance class.
type TerminalThemeMode string

const (
	ThemeModeLight TerminalThemeMode = "light"
	ThemeModeDark  TerminalThemeMode = "dark"
)

// AnnotationConfidence is the confidence level of an agent annotation.
type AnnotationConfidence string

const (
	ConfidenceLow    AnnotationConfidence = "low"
	ConfidenceMedium AnnotationConfidence = "medium"
	ConfidenceHigh   AnnotationConfidence = "high"
)

// FileChangeType describes how a file was changed.
type FileChangeType string

const (
	FileChangeNew           FileChangeType = "new"
	FileChangeDeleted       FileChangeType = "deleted"
	FileChangeChange        FileChangeType = "change"
	FileChangeRenamePure    FileChangeType = "rename-pure"
	FileChangeRenameChanged FileChangeType = "rename-changed"
)

// AgentAnnotation is one AI-generated note attached to a range of diff lines.
type AgentAnnotation struct {
	ID         *string               `json:"id,omitempty"`
	OldRange   *[2]int               `json:"oldRange,omitempty"`
	NewRange   *[2]int               `json:"newRange,omitempty"`
	Summary    string                `json:"summary"`
	Rationale  *string               `json:"rationale,omitempty"`
	Tags       []string              `json:"tags,omitempty"`
	Confidence *AnnotationConfidence `json:"confidence,omitempty"`
	Source     *string               `json:"source,omitempty"`
	Author     *string               `json:"author,omitempty"`
	CreatedAt  *string               `json:"createdAt,omitempty"`
}

// AgentFileContext holds annotations for one file from a review sidecar.
type AgentFileContext struct {
	Path        string            `json:"path"`
	Summary     *string           `json:"summary,omitempty"`
	Annotations []AgentAnnotation `json:"annotations"`
}

// AgentContext is the top-level structure of a review sidecar JSON file.
type AgentContext struct {
	Version int                `json:"version"`
	Summary *string            `json:"summary,omitempty"`
	Files   []AgentFileContext `json:"files"`
}

// DiffStats summarises addition/deletion counts for one file.
type DiffStats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

// DiffUnk is one contiguous block of changes inside a file diff.
type DiffUnk struct {
	Index    int     `json:"index"`
	Header   string  `json:"header"`
	OldRange *[2]int `json:"oldRange,omitempty"`
	NewRange *[2]int `json:"newRange,omitempty"`
}

// DiffMetadata is the parsed structural metadata for one file diff.
type DiffMetadata struct {
	Name             string         `json:"name"`
	PrevName         *string        `json:"prevName,omitempty"`
	Type             FileChangeType `json:"type"`
	Mode             *string        `json:"mode,omitempty"`     // new file mode (e.g. "100755")
	PrevMode         *string        `json:"prevMode,omitempty"` // old file mode for mode-change diffs
	Unks             []DiffUnk      `json:"unks"`
	SplitLineCount   int            `json:"splitLineCount"`
	UnifiedLineCount int            `json:"unifiedLineCount"`
	IsPartial        bool           `json:"isPartial"`
	AdditionLines    []int          `json:"additionLines"`
	DeletionLines    []int          `json:"deletionLines"`
	CacheKey         string         `json:"cacheKey"`
}

// DiffFile is one file in a review changeset.
type DiffFile struct {
	ID             string            `json:"id"`
	Path           string            `json:"path"`
	PreviousPath   *string           `json:"previousPath,omitempty"`
	Patch          string            `json:"patch"`
	Language       *string           `json:"language,omitempty"`
	Stats          DiffStats         `json:"stats"`
	Metadata       DiffMetadata      `json:"metadata"`
	Agent          *AgentFileContext  `json:"agent"`
	IsUntracked    bool              `json:"isUntracked,omitempty"`
	IsBinary       bool              `json:"isBinary,omitempty"`
	IsTooLarge     bool              `json:"isTooLarge,omitempty"`
	StatsTruncated bool              `json:"statsTruncated,omitempty"`
}

// Changeset is the normalized representation of a diff review session.
type Changeset struct {
	ID           string     `json:"id"`
	SourceLabel  string     `json:"sourceLabel"`
	Title        string     `json:"title"`
	Summary      *string    `json:"summary,omitempty"`
	AgentSummary *string    `json:"agentSummary,omitempty"`
	Files        []DiffFile `json:"files"`
}
