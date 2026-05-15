package config

import "github.com/kpango/unk/internal/types"

// Resolver resolves the layered configuration for a unk invocation.
// Use Default() to obtain the production implementation backed by the
// package-level Resolve function, or substitute a mock in tests.
type Resolver interface {
	Resolve(input types.CLIInput, cwd string) (*Resolution, error)
}

type defaultResolver struct{}

// Default returns a Resolver backed by the package-level Resolve function.
func Default() Resolver { return defaultResolver{} }

func (defaultResolver) Resolve(input types.CLIInput, cwd string) (*Resolution, error) {
	return Resolve(input, cwd)
}
