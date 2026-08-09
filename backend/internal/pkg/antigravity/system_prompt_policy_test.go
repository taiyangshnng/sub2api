package antigravity

import "testing"

func TestNormalizeSystemPromptStrategy(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  SystemPromptStrategy
		err   bool
	}{
		{name: "empty defaults to append", input: "", want: SystemPromptStrategyAppend},
		{name: "whitespace and case normalize", input: " AUTHORITATIVE ", want: SystemPromptStrategyAuthoritative},
		{name: "ignore", input: "ignore", want: SystemPromptStrategyIgnore},
		{name: "unknown", input: "replace", err: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeSystemPromptStrategy(tt.input)
			if (err != nil) != tt.err {
				t.Fatalf("error=%v, want error=%v", err, tt.err)
			}
			if !tt.err && got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSystemPromptPolicyMatrix(t *testing.T) {
	tests := []struct {
		name       string
		strategy   SystemPromptStrategy
		identity   bool
		mcp        bool
		wantPolicy SystemPromptPolicy
	}{
		{
			name:     "append identity on mcp on",
			strategy: SystemPromptStrategyAppend, identity: true, mcp: true,
			wantPolicy: SystemPromptPolicy{PreserveClientSystem: true, InjectIdentityPatch: true, InjectModelIdentity: true, InjectIsolationGuard: true, ApplyOpenCodeFilter: true, EnableMCPXML: true, AddSystemPromptEnd: true},
		},
		{
			name:     "append identity off",
			strategy: SystemPromptStrategyAppend, identity: false, mcp: true,
			wantPolicy: SystemPromptPolicy{PreserveClientSystem: true, ApplyOpenCodeFilter: true, EnableMCPXML: true, AddSystemPromptEnd: true},
		},
		{
			name:     "ignore identity on",
			strategy: SystemPromptStrategyIgnore, identity: true, mcp: true,
			wantPolicy: SystemPromptPolicy{InjectIdentityPatch: true, InjectModelIdentity: true, InjectIsolationGuard: true, EnableMCPXML: true, AddSystemPromptEnd: true},
		},
		{
			name:     "ignore identity off",
			strategy: SystemPromptStrategyIgnore, identity: false, mcp: false,
			wantPolicy: SystemPromptPolicy{AddSystemPromptEnd: true},
		},
		{
			name:     "authoritative identity on",
			strategy: SystemPromptStrategyAuthoritative, identity: true, mcp: true,
			wantPolicy: SystemPromptPolicy{PreserveClientSystem: true},
		},
		{
			name:     "authoritative identity off",
			strategy: SystemPromptStrategyAuthoritative, identity: false, mcp: false,
			wantPolicy: SystemPromptPolicy{PreserveClientSystem: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSystemPromptPolicy(tt.strategy, tt.identity, tt.mcp)
			if got != tt.wantPolicy {
				t.Fatalf("got %#v, want %#v", got, tt.wantPolicy)
			}
		})
	}
}
