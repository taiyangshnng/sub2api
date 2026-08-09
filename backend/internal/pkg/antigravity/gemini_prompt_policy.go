package antigravity

import (
	"encoding/json"
	"strings"
)

// ApplyGeminiPromptPolicy applies the request-local prompt policy to a native
// Gemini body. Protocol fields and client-owned tools are preserved; only
// Sub2API-owned prompt workarounds are controlled here.
func ApplyGeminiPromptPolicy(body []byte, modelName string, policy SystemPromptPolicy) ([]byte, error) {
	return ApplyGeminiPromptPolicyWithIdentity(body, modelName, policy, "")
}

// ApplyGeminiPromptPolicyWithIdentity is the native Gemini equivalent of the
// Claude transform policy hook. identityPatch is caller-owned configuration;
// an empty value uses the built-in compatibility identity.
func ApplyGeminiPromptPolicyWithIdentity(body []byte, modelName string, policy SystemPromptPolicy, identityPatch string) ([]byte, error) {
	var request map[string]any
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}

	system, systemKey := geminiSystemInstruction(request)
	if !policy.PreserveClientSystem {
		for _, key := range geminiSystemInstructionKeys {
			delete(request, key)
		}
		system = nil
		systemKey = ""
	}
	parts := geminiSystemParts(system)
	userHasIdentity := geminiPartsHaveIdentity(parts)
	if policy.PreserveClientSystem && policy.ApplyOpenCodeFilter {
		parts = filterGeminiSystemParts(parts)
	}

	if !userHasIdentity {
		if policy.InjectIdentityPatch {
			if strings.TrimSpace(identityPatch) == "" {
				identityPatch = GetDefaultIdentityPatch()
			}
			parts = append([]any{map[string]any{"text": identityPatch}}, parts...)
		}
		if policy.InjectIsolationGuard {
			parts = append(parts, map[string]any{"text": isolationGuardPrompt})
		}
		if policy.InjectModelIdentity {
			if modelIdentity := BuildModelIdentityText(modelName); modelIdentity != "" {
				parts = append(parts, map[string]any{"text": modelIdentity})
			}
		}
	}

	if policy.EnableMCPXML && geminiBodyHasMCPTools(request) {
		parts = append(parts, map[string]any{"text": mcpXMLProtocol})
	}
	if policy.AddSystemPromptEnd && !userHasIdentity && len(parts) > 0 {
		parts = append(parts, map[string]any{"text": "\n--- [SYSTEM_PROMPT_END] ---"})
	}

	if len(parts) > 0 {
		if system == nil {
			request["systemInstruction"] = map[string]any{"parts": parts}
		} else {
			// Keep native Gemini metadata (for example role) intact while
			// changing only the prompt parts owned by this policy.
			system["parts"] = parts
			request[systemKey] = system
		}
	} else if system != nil && policy.PreserveClientSystem && policy.ApplyOpenCodeFilter {
		// Legacy filtering drops an OpenCode wrapper when it contains no client
		// instruction. Do not leave the unfiltered native field behind.
		delete(request, systemKey)
	}
	return json.Marshal(request)
}

var geminiSystemInstructionKeys = []string{"systemInstruction", "system_instruction", "_system_instruction"}

func geminiSystemInstruction(request map[string]any) (map[string]any, string) {
	for _, key := range geminiSystemInstructionKeys {
		if system, ok := request[key].(map[string]any); ok {
			return system, key
		}
	}
	return nil, ""
}

func geminiSystemParts(system map[string]any) []any {
	if system == nil {
		return nil
	}
	parts, _ := system["parts"].([]any)
	return append([]any(nil), parts...)
}

func geminiPartsHaveIdentity(parts []any) bool {
	for _, part := range parts {
		partMap, ok := part.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := partMap["text"].(string); ok && strings.Contains(text, "You are Antigravity") {
			return true
		}
	}
	return false
}

func filterGeminiSystemParts(parts []any) []any {
	filtered := make([]any, 0, len(parts))
	for _, part := range parts {
		partMap, ok := part.(map[string]any)
		if !ok {
			filtered = append(filtered, part)
			continue
		}
		text, hasText := partMap["text"].(string)
		if !hasText {
			filtered = append(filtered, part)
			continue
		}
		text = filterOpenCodePrompt(text)
		if text != "" {
			partMap["text"] = text
			filtered = append(filtered, partMap)
		}
	}
	return filtered
}

func geminiBodyHasMCPTools(request map[string]any) bool {
	tools, _ := request["tools"].([]any)
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"functionDeclarations", "function_declarations"} {
			declarations, _ := tool[key].([]any)
			for _, rawDeclaration := range declarations {
				declaration, _ := rawDeclaration.(map[string]any)
				name, _ := declaration["name"].(string)
				if strings.HasPrefix(name, "mcp__") {
					return true
				}
			}
		}
	}
	return false
}
