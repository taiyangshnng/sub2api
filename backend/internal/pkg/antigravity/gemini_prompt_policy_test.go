package antigravity

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func decodeGeminiPolicyBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var request map[string]any
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	return request
}

func TestApplyGeminiPromptPolicyAuthoritativePreservesClientRequest(t *testing.T) {
	input := map[string]any{
		"systemInstruction": map[string]any{
			"role":  "user",
			"parts": []any{map[string]any{"text": "You are an interactive CLI tool. Keep this client prompt."}},
		},
		"contents": []any{
			map[string]any{"role": "user", "parts": []any{map[string]any{"text": "hello"}}},
			map[string]any{"role": "model", "parts": []any{map[string]any{"functionCall": map[string]any{"name": "mcp__read"}}}},
			map[string]any{"role": "user", "parts": []any{map[string]any{"functionResponse": map[string]any{"name": "mcp__read"}}}},
		},
		"tools": []any{map[string]any{"functionDeclarations": []any{map[string]any{"name": "mcp__read"}}}},
	}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	policy := ResolveSystemPromptPolicy(SystemPromptStrategyAuthoritative, true, true)
	output, err := ApplyGeminiPromptPolicyWithIdentity(body, "claude-opus-4-5", policy, "custom identity")
	if err != nil {
		t.Fatal(err)
	}
	got := decodeGeminiPolicyBody(t, output)

	if !reflect.DeepEqual(got["systemInstruction"], input["systemInstruction"]) {
		t.Fatalf("authoritative changed client systemInstruction: got %#v", got["systemInstruction"])
	}
	if !reflect.DeepEqual(got["contents"], input["contents"]) {
		t.Fatalf("authoritative changed client contents: got %#v", got["contents"])
	}
	if !reflect.DeepEqual(got["tools"], input["tools"]) {
		t.Fatalf("authoritative changed client tools: got %#v", got["tools"])
	}
	encoded, _ := json.Marshal(got)
	text := string(encoded)
	for _, unwanted := range []string{"You are Antigravity", "MCP XML", "SYSTEM_PROMPT_END", "custom identity"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("authoritative injected %q: %s", unwanted, text)
		}
	}
}

func TestApplyGeminiPromptPolicyIgnoreDropsOnlyClientSystem(t *testing.T) {
	input := map[string]any{
		"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": "client-only system"}}},
		"contents": []any{
			map[string]any{"role": "user", "parts": []any{map[string]any{"text": "question"}}},
			map[string]any{"role": "model", "parts": []any{map[string]any{"functionCall": map[string]any{"name": "lookup"}}}},
			map[string]any{"role": "user", "parts": []any{map[string]any{"functionResponse": map[string]any{"name": "lookup"}}}},
		},
		"tools": []any{map[string]any{"functionDeclarations": []any{map[string]any{"name": "lookup"}}}},
	}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	policy := ResolveSystemPromptPolicy(SystemPromptStrategyIgnore, false, false)
	output, err := ApplyGeminiPromptPolicy(body, "gemini-2.5-flash", policy)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeGeminiPolicyBody(t, output)

	if _, ok := got["systemInstruction"]; ok {
		t.Fatalf("ignore with identity and MCP disabled should remove client systemInstruction: %#v", got["systemInstruction"])
	}
	if !reflect.DeepEqual(got["contents"], input["contents"]) {
		t.Fatalf("ignore changed contents: got %#v", got["contents"])
	}
	if !reflect.DeepEqual(got["tools"], input["tools"]) {
		t.Fatalf("ignore changed client tools: got %#v", got["tools"])
	}
}

func TestApplyGeminiPromptPolicyIgnoreStillInjectsIdentityWhenEnabled(t *testing.T) {
	input := []byte(`{"systemInstruction":{"parts":[{"text":"client-only system"}]},"contents":[{"role":"user","parts":[{"text":"question"}]}]}`)
	policy := ResolveSystemPromptPolicy(SystemPromptStrategyIgnore, true, false)

	output, err := ApplyGeminiPromptPolicy(input, "gemini-2.5-flash", policy)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeGeminiPolicyBody(t, output)
	system, ok := got["systemInstruction"].(map[string]any)
	if !ok {
		t.Fatalf("identity-enabled ignore dropped generated systemInstruction: %#v", got["systemInstruction"])
	}
	parts, ok := system["parts"].([]any)
	if !ok || len(parts) == 0 {
		t.Fatalf("identity-enabled ignore did not create prompt parts: %#v", system)
	}
	first, _ := parts[0].(map[string]any)
	text, _ := first["text"].(string)
	if !strings.Contains(text, "You are Antigravity") {
		t.Fatalf("identity-enabled ignore did not inject identity: %q", text)
	}
	if strings.Contains(string(output), "client-only system") {
		t.Fatalf("ignore retained client system prompt: %s", output)
	}
}

func TestApplyGeminiPromptPolicyAuthoritativePreservesOpenCodePrompt(t *testing.T) {
	input := []byte(`{"systemInstruction":{"role":"user","parts":[{"text":"You are an interactive CLI tool. Instructions from: project rules"}]}}`)
	policy := ResolveSystemPromptPolicy(SystemPromptStrategyAuthoritative, false, false)
	output, err := ApplyGeminiPromptPolicy(input, "gemini-2.5-flash", policy)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "You are an interactive CLI tool") {
		t.Fatalf("authoritative unexpectedly filtered OpenCode prompt: %s", output)
	}
}

func TestApplyGeminiPromptPolicyAppendPreservesLegacyNativeOpenCodePrompt(t *testing.T) {
	input := []byte(`{"systemInstruction":{"role":"user","parts":[{"text":"You are an interactive CLI tool. Instructions from: project rules"}]}}`)
	policy := ResolveSystemPromptPolicy(SystemPromptStrategyAppend, false, false)
	output, err := ApplyGeminiPromptPolicy(input, "gemini-2.5-flash", policy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), "You are an interactive CLI tool") {
		t.Fatalf("native append did not apply legacy OpenCode filtering: %s", output)
	}
	if !strings.Contains(string(output), "Instructions from: project rules") {
		t.Fatalf("native append dropped the client instruction preserved by legacy filtering: %s", output)
	}
}

func TestApplyGeminiPromptPolicyAppendAddsCompatibilityComponents(t *testing.T) {
	input := []byte(`{"systemInstruction":{"role":"user","parts":[{"text":"You are an interactive CLI tool. Instructions from: keep this rule"}]},"tools":[{"functionDeclarations":[{"name":"mcp__read"}]}]}`)
	policy := ResolveSystemPromptPolicy(SystemPromptStrategyAppend, true, true)

	output, err := ApplyGeminiPromptPolicyWithIdentity(input, "claude-opus-4-5", policy, "custom identity")
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, required := range []string{"custom identity", "You are Model Claude Opus 4.5", "MCP XML", "SYSTEM_PROMPT_END", "Instructions from: keep this rule"} {
		if !strings.Contains(text, required) {
			t.Fatalf("append omitted %q: %s", required, text)
		}
	}
	if strings.Contains(text, "You are an interactive CLI tool") {
		t.Fatalf("append retained the filtered OpenCode wrapper: %s", text)
	}
}

func TestApplyGeminiPromptPolicyIdentityOffWithoutClientSystemDoesNotCreateSystemInstruction(t *testing.T) {
	input, err := json.Marshal(map[string]any{
		"contents": []any{map[string]any{
			"role":  "user",
			"parts": []any{map[string]any{"text": "question"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	policy := ResolveSystemPromptPolicy(SystemPromptStrategyAppend, false, false)
	output, err := ApplyGeminiPromptPolicy(input, "gemini-2.5-flash", policy)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeGeminiPolicyBody(t, output)
	if _, ok := got["systemInstruction"]; ok {
		t.Fatalf("identity-off request unexpectedly created systemInstruction: %s", output)
	}
}

func TestApplyGeminiPromptPolicyAuthoritativePreservesAlternateSystemInstructionKey(t *testing.T) {
	input, err := json.Marshal(map[string]any{
		"system_instruction": map[string]any{
			"role":  "user",
			"parts": []any{map[string]any{"text": "client system"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	policy := ResolveSystemPromptPolicy(SystemPromptStrategyAuthoritative, true, true)
	output, err := ApplyGeminiPromptPolicy(input, "gemini-2.5-flash", policy)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeGeminiPolicyBody(t, output)
	if _, ok := got["system_instruction"]; !ok {
		t.Fatalf("authoritative did not preserve alternate system key: %s", output)
	}
	text := string(output)
	for _, unwanted := range []string{"You are Antigravity", "SYSTEM_PROMPT_END", "MCP XML"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("authoritative injected %q into alternate system key request: %s", unwanted, output)
		}
	}
}
