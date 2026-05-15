package types

// VCSInput is the normalised input for `unk diff` VCS reviews.
type VCSInput struct {
	OptionCarrier
	Range     *string
	Staged    bool
	Pathspecs []string
}

func (VCSInput) Kind() string                             { return "vcs" }
func (v VCSInput) SetOptions(opts CommonOptions) CLIInput { v.Options = opts; return &v }

// ShowInput is the normalised input for `unk show` commit reviews.
type ShowInput struct {
	OptionCarrier
	Ref       *string
	Pathspecs []string
}

func (ShowInput) Kind() string                             { return "show" }
func (v ShowInput) SetOptions(opts CommonOptions) CLIInput { v.Options = opts; return &v }

// StashShowInput is the normalised input for `unk stash show`.
type StashShowInput struct {
	OptionCarrier
	Ref *string
}

func (StashShowInput) Kind() string                             { return "stash-show" }
func (v StashShowInput) SetOptions(opts CommonOptions) CLIInput { v.Options = opts; return &v }

// FileInput is the normalised input for `unk diff <left> <right>`.
type FileInput struct {
	OptionCarrier
	Left  string
	Right string
}

func (FileInput) Kind() string                             { return "diff" }
func (v FileInput) SetOptions(opts CommonOptions) CLIInput { v.Options = opts; return &v }

// PatchInput is the normalised input for `unk patch [file]`.
type PatchInput struct {
	OptionCarrier
	File *string
	Text *string
}

func (PatchInput) Kind() string                             { return "patch" }
func (v PatchInput) SetOptions(opts CommonOptions) CLIInput { v.Options = opts; return &v }

// DiffToolInput is the normalised input for `unk difftool <left> <right>`.
type DiffToolInput struct {
	OptionCarrier
	Left  string
	Right string
	Path  *string
}

func (DiffToolInput) Kind() string                             { return "difftool" }
func (v DiffToolInput) SetOptions(opts CommonOptions) CLIInput { v.Options = opts; return &v }

// HelpInput is returned when --help is requested. The string value is the
// pre-formatted help text.
type HelpInput string

func (HelpInput) Kind() string                          { return "help" }
func (HelpInput) GetOptions() CommonOptions             { return CommonOptions{} }
func (h HelpInput) SetOptions(_ CommonOptions) CLIInput { return h }

// PagerInput is the normalised input for `unk pager`. The underlying value
// holds the options directly.
type PagerInput CommonOptions

func (PagerInput) Kind() string                             { return "pager" }
func (v PagerInput) GetOptions() CommonOptions              { return CommonOptions(v) }
func (v PagerInput) SetOptions(opts CommonOptions) CLIInput { p := PagerInput(opts); return &p }
