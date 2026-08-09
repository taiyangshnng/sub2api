package antigravity

import (
	"fmt"
	"strings"
)

// SystemPromptStrategy controls how client-owned system instructions interact
// with Sub2API's Antigravity compatibility prompt.
type SystemPromptStrategy string

const (
	SystemPromptStrategyAppend        SystemPromptStrategy = "append"
	SystemPromptStrategyIgnore        SystemPromptStrategy = "ignore"
	SystemPromptStrategyAuthoritative SystemPromptStrategy = "authoritative"
)

// NormalizeSystemPromptStrategy normalizes omitted values to the legacy append
// behavior and rejects values that cannot be persisted safely.
func NormalizeSystemPromptStrategy(value string) (SystemPromptStrategy, error) {
	switch SystemPromptStrategy(strings.ToLower(strings.TrimSpace(value))) {
	case "", SystemPromptStrategyAppend:
		return SystemPromptStrategyAppend, nil
	case SystemPromptStrategyIgnore:
		return SystemPromptStrategyIgnore, nil
	case SystemPromptStrategyAuthoritative:
		return SystemPromptStrategyAuthoritative, nil
	default:
		return "", fmt.Errorf("invalid system prompt strategy %q", value)
	}
}

// SystemPromptPolicy is the request-local prompt policy consumed by all
// Antigravity protocol transformers. It has no repository or context
// dependencies so the precedence rules remain independently testable.
type SystemPromptPolicy struct {
	PreserveClientSystem bool

	InjectIdentityPatch  bool
	InjectModelIdentity  bool
	InjectIsolationGuard bool

	ApplyOpenCodeFilter bool
	EnableMCPXML        bool
	AddSystemPromptEnd  bool
}

// ResolveSystemPromptPolicy applies Group strategy > global identity setting
// > legacy defaults. The ignore strategy only changes treatment of client
// system instructions; the authoritative strategy disables Sub2API-owned
// prompt workarounds entirely.
func ResolveSystemPromptPolicy(
	strategy SystemPromptStrategy,
	globalIdentityPatch bool,
	groupMCPXML bool,
) SystemPromptPolicy {
	normalized, err := NormalizeSystemPromptStrategy(string(strategy))
	if err != nil {
		normalized = SystemPromptStrategyAppend
	}

	policy := SystemPromptPolicy{
		PreserveClientSystem: true,
		InjectIdentityPatch:  globalIdentityPatch,
		InjectModelIdentity:  globalIdentityPatch,
		InjectIsolationGuard: globalIdentityPatch,
		ApplyOpenCodeFilter:  true,
		EnableMCPXML:         groupMCPXML,
		AddSystemPromptEnd:   true,
	}

	switch normalized {
	case SystemPromptStrategyIgnore:
		policy.PreserveClientSystem = false
		policy.ApplyOpenCodeFilter = false
	case SystemPromptStrategyAuthoritative:
		policy.InjectIdentityPatch = false
		policy.InjectModelIdentity = false
		policy.InjectIsolationGuard = false
		policy.ApplyOpenCodeFilter = false
		policy.EnableMCPXML = false
		policy.AddSystemPromptEnd = false
	}

	return policy
}
