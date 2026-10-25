package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode"

	"promptgate/backend/internal/platform/proxylimits"
)

type MethodGuardOptions struct {
	MaxBufferedRequestBytes int64
}

// MethodGuardMiddleware rejects unsupported methods for every route Prompt
// Gate exposes deliberately. It must run before policy, quota, and bridge
// handlers so invalid methods never consume a body or reach an upstream
// provider.
func MethodGuardMiddleware(next http.Handler) http.Handler {
	return MethodGuardMiddlewareWithOptions(MethodGuardOptions{})(next)
}

// MethodGuardMiddlewareWithOptions rejects routes, methods, and request bodies
// that could escape Prompt Gate's user boundary. Provider credentials are
// shared, so references to provider-owned objects cannot be forwarded until
// Prompt Gate has explicit per-user ownership records for them.
func MethodGuardMiddlewareWithOptions(options MethodGuardOptions) func(http.Handler) http.Handler {
	maxRequestBytes := options.MaxBufferedRequestBytes
	if maxRequestBytes <= 0 {
		maxRequestBytes = proxylimits.DefaultMaxBufferedRequestBytes
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowedMethod, guarded := guardedProviderRouteMethod(r.URL.Path)
			if !guarded {
				if _, providerScoped := providerRoutePath(r.URL.Path); providerScoped {
					writeProviderRouteNotFound(w)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			if r.Method != allowedMethod {
				writeMethodNotAllowed(w, allowedMethod)
				return
			}
			if allowedMethod == http.MethodPost {
				if err := validateProviderRequestBody(r, maxRequestBytes); err != nil {
					switch {
					case errors.Is(err, proxylimits.ErrExceeded):
						writeProviderRequestError(w, http.StatusRequestEntityTooLarge, "request_body_too_large")
					case errors.Is(err, errProviderStateReference):
						writeProviderRequestError(w, http.StatusBadRequest, "provider_state_reference_not_allowed")
					default:
						writeProviderRequestError(w, http.StatusBadRequest, "invalid_request_body")
					}
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

var errProviderStateReference = errors.New("provider state reference is not allowed")

func validateProviderRequestBody(r *http.Request, maxRequestBytes int64) error {
	if r.Body == nil {
		return io.ErrUnexpectedEOF
	}
	raw, err := proxylimits.ReadAll(r.Body, maxRequestBytes)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return err
	}

	decoded, err := decodeStrictJSON(raw)
	if err != nil {
		return errors.New("request body must be one JSON object")
	}
	payload, ok := decoded.(map[string]any)
	if !ok || payload == nil {
		return errors.New("request body must be one JSON object")
	}
	providerPath, _ := providerRoutePath(r.URL.Path)
	if containsProviderStateReference(payload, providerPath, true) {
		return errProviderStateReference
	}
	return nil
}

func decodeStrictJSON(raw []byte) (any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeStrictJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}

func decodeStrictJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return token, nil
	}

	switch delim {
	case '{':
		object := make(map[string]any)
		seenKeys := make(map[string]string)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("JSON object key is not a string")
			}
			foldedKey := simpleFoldJSONKey(key)
			if previous, exists := seenKeys[foldedKey]; exists {
				return nil, errors.New("duplicate JSON object key " + previous)
			}
			seenKeys[foldedKey] = key
			child, err := decodeStrictJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = child
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, errors.New("unterminated JSON object")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			child, err := decodeStrictJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, child)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, errors.New("unterminated JSON array")
		}
		return array, nil
	default:
		return nil, errors.New("unexpected JSON delimiter")
	}
}

// simpleFoldJSONKey returns one stable representative for every Unicode
// simple-fold orbit. Using it as a map key keeps duplicate detection linear in
// the number of object keys while retaining strings.EqualFold semantics (for
// example, "s" and the long-s rune U+017F collide).
func simpleFoldJSONKey(value string) string {
	var folded strings.Builder
	folded.Grow(len(value))
	for _, current := range value {
		minimum := current
		for candidate := unicode.SimpleFold(current); candidate != current; candidate = unicode.SimpleFold(candidate) {
			if candidate < minimum {
				minimum = candidate
			}
		}
		folded.WriteRune(minimum)
	}
	return folded.String()
}

func containsProviderStateReference(value any, providerPath string, root bool) bool {
	payload, ok := value.(map[string]any)
	if !ok || !root {
		return false
	}
	switch providerPath {
	case "/v1/responses":
		return responsesRequestHasProviderState(payload)
	case "/v1/chat/completions":
		return chatRequestHasProviderState(payload)
	case "/v1/messages", "/v1/messages/count_tokens":
		return anthropicRequestHasProviderState(payload)
	default:
		return false
	}
}

func responsesRequestHasProviderState(payload map[string]any) bool {
	for _, key := range []string{"previous_response_id", "conversation", "prompt"} {
		if meaningfulJSONValue(jsonObjectValue(payload, key)) {
			return true
		}
	}
	if responsesInputHasProviderState(jsonObjectValue(payload, "input")) {
		return true
	}
	tools, _ := jsonObjectValue(payload, "tools").([]any)
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			return true
		}
		typeName := objectType(tool)
		switch typeName {
		case "file_search":
			// File search is necessarily backed by provider-owned vector stores.
			return true
		case "code_interpreter":
			if unsafeContainerReference(jsonObjectValue(tool, "container")) || meaningfulJSONValue(jsonObjectValue(tool, "file_ids")) || meaningfulJSONValue(jsonObjectValue(tool, "upload_ids")) || toolSkillsHaveProviderReferences(jsonObjectValue(tool, "skills")) {
				return true
			}
		case "shell":
			if shellToolHasProviderState(tool) {
				return true
			}
		case "mcp":
			// Both remote servers and provider-managed connectors can expose
			// account-scoped data through the shared provider credential.
			return true
		case "image_generation":
			// Image generation can select a separately-authorized model and
			// consume provider-owned masks/files. It needs dedicated ownership,
			// model authorization, and accounting before it can be enabled.
			return true
		case "namespace":
			if namespaceToolHasProviderState(tool) {
				return true
			}
		case "function", "custom", "local_shell", "apply_patch", "tool_search",
			"web_search", "web_search_2025_08_26", "web_search_preview", "web_search_preview_2025_03_11",
			"computer", "computer_use_preview":
			// Explicitly reviewed request-local tool definitions.
		case "":
			return true
		default:
			// New tool variants fail closed until their ownership model is known.
			return true
		}
	}
	return false
}

func shellToolHasProviderState(tool map[string]any) bool {
	environment, ok := jsonObjectValue(tool, "environment").(map[string]any)
	if !ok || len(environment) == 0 {
		// The provider may choose a hosted default when no environment is given.
		return true
	}
	switch objectType(environment) {
	case "local":
		return false
	case "container_auto":
		return meaningfulJSONValue(jsonObjectValue(environment, "file_ids")) ||
			toolSkillsHaveProviderReferences(jsonObjectValue(environment, "skills"))
	case "container_reference":
		return true
	default:
		return true
	}
}

func toolSkillsHaveProviderReferences(value any) bool {
	skills, _ := value.([]any)
	for _, rawSkill := range skills {
		skill, ok := rawSkill.(map[string]any)
		if !ok || objectType(skill) != "inline" {
			return true
		}
	}
	return false
}

func namespaceToolHasProviderState(tool map[string]any) bool {
	nestedTools, ok := jsonObjectValue(tool, "tools").([]any)
	if !ok {
		return true
	}
	for _, rawNested := range nestedTools {
		nested, ok := rawNested.(map[string]any)
		if !ok {
			return true
		}
		switch objectType(nested) {
		case "function", "custom":
		default:
			return true
		}
	}
	return false
}

func responsesInputHasProviderState(value any) bool {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if responsesInputHasProviderState(item) {
				return true
			}
		}
	case map[string]any:
		switch objectType(typed) {
		case "item_reference", "file_reference", "response_reference", "conversation_reference":
			return meaningfulJSONValue(jsonObjectValue(typed, "id"))
		case "input_file":
			return meaningfulJSONValue(jsonObjectValue(typed, "file_id")) || meaningfulJSONValue(jsonObjectValue(typed, "upload_id")) || remoteURLReference(jsonObjectValue(typed, "file_url"))
		case "input_image", "computer_screenshot":
			return meaningfulJSONValue(jsonObjectValue(typed, "file_id")) || remoteURLReference(jsonObjectValue(typed, "image_url"))
		case "reasoning", "compaction":
			return meaningfulJSONValue(jsonObjectValue(typed, "id")) || meaningfulJSONValue(jsonObjectValue(typed, "encrypted_content"))
		case "message":
			role, _ := jsonObjectValue(typed, "role").(string)
			if strings.EqualFold(strings.TrimSpace(role), "assistant") || meaningfulJSONValue(jsonObjectValue(typed, "id")) {
				return true
			}
			return responsesInputHasProviderState(jsonObjectValue(typed, "content"))
		case "function_call", "custom_tool_call":
			// call_id correlates caller-executed local tools. The provider item
			// ID, when present, is account-scoped state and cannot be replayed.
			return meaningfulJSONValue(jsonObjectValue(typed, "id"))
		case "function_call_output", "custom_tool_call_output":
			return meaningfulJSONValue(jsonObjectValue(typed, "id")) || responsesInputHasProviderState(jsonObjectValue(typed, "output"))
		case "input_text", "output_text", "text", "refusal":
			return false
		case "":
			return responsesInputHasProviderState(jsonObjectValue(typed, "content"))
		case "file_search_call", "computer_call", "computer_call_output", "web_search_call",
			"tool_search_call", "tool_search_output", "image_generation_call", "code_interpreter_call",
			"local_shell_call", "local_shell_call_output", "shell_call", "shell_call_output",
			"apply_patch_call", "apply_patch_call_output", "mcp_list_tools", "mcp_approval_request",
			"mcp_approval_response", "mcp_call":
			// These variants are generated by provider/server-side tools and
			// contain IDs or outputs owned by the provider account. Treat the
			// whole variant as unsafe rather than trying to maintain a partial
			// list of fields for an evolving union.
			return true
		default:
			// The Responses input union grows regularly. Unknown typed items
			// fail closed until their state and ownership semantics are reviewed.
			return true
		}
	}
	return false
}

func chatRequestHasProviderState(payload map[string]any) bool {
	messages, _ := jsonObjectValue(payload, "messages").([]any)
	for _, rawMessage := range messages {
		message, _ := rawMessage.(map[string]any)
		if audio, ok := jsonObjectValue(message, "audio").(map[string]any); ok && meaningfulJSONValue(jsonObjectValue(audio, "id")) {
			return true
		}
		attachments, _ := jsonObjectValue(message, "attachments").([]any)
		for _, rawAttachment := range attachments {
			attachment, _ := rawAttachment.(map[string]any)
			if meaningfulJSONValue(jsonObjectValue(attachment, "file_id")) {
				return true
			}
		}
		content, _ := jsonObjectValue(message, "content").([]any)
		for _, rawPart := range content {
			part, ok := rawPart.(map[string]any)
			if !ok {
				return true
			}
			switch objectType(part) {
			case "text", "input_audio":
				continue
			case "image_url", "input_image":
				if remoteURLReference(jsonObjectValue(part, "image_url")) || meaningfulJSONValue(jsonObjectValue(part, "file_id")) {
					return true
				}
			case "file", "input_file":
				file, _ := jsonObjectValue(part, "file").(map[string]any)
				if meaningfulJSONValue(jsonObjectValue(part, "file_id")) || meaningfulJSONValue(jsonObjectValue(file, "file_id")) || meaningfulJSONValue(jsonObjectValue(part, "upload_id")) || meaningfulJSONValue(jsonObjectValue(file, "upload_id")) || remoteURLReference(jsonObjectValue(part, "file_url")) || remoteURLReference(jsonObjectValue(file, "file_url")) {
					return true
				}
			default:
				return true
			}
		}
	}
	return false
}

func anthropicRequestHasProviderState(payload map[string]any) bool {
	if meaningfulJSONValue(jsonObjectValue(payload, "container")) {
		return true
	}
	messages, _ := jsonObjectValue(payload, "messages").([]any)
	for _, rawMessage := range messages {
		message, _ := rawMessage.(map[string]any)
		if anthropicContentHasProviderState(jsonObjectValue(message, "content")) {
			return true
		}
	}
	return false
}

func anthropicContentHasProviderState(value any) bool {
	switch typed := value.(type) {
	case []any:
		for _, rawPart := range typed {
			if anthropicContentHasProviderState(rawPart) {
				return true
			}
		}
	case map[string]any:
		switch objectType(typed) {
		case "text":
			return false
		case "image":
			source, _ := jsonObjectValue(typed, "source").(map[string]any)
			switch objectType(source) {
			case "url":
				return remoteURLReference(jsonObjectValue(source, "url"))
			case "file":
				return meaningfulJSONValue(jsonObjectValue(source, "file_id"))
			case "base64", "":
				return false
			default:
				return true
			}
		case "document":
			source, _ := jsonObjectValue(typed, "source").(map[string]any)
			switch objectType(source) {
			case "url":
				return remoteURLReference(jsonObjectValue(source, "url"))
			case "file":
				return meaningfulJSONValue(jsonObjectValue(source, "file_id"))
			case "content":
				return anthropicContentHasProviderState(jsonObjectValue(source, "content"))
			case "base64", "text", "":
				return false
			default:
				return true
			}
		case "tool_result":
			return anthropicContentHasProviderState(jsonObjectValue(typed, "content"))
		case "tool_use":
			// Tool input is arbitrary caller-owned JSON. It may legitimately
			// contain business fields such as customer_id or url and is not a
			// provider object reference namespace.
			return false
		case "thinking":
			// Thinking blocks are provider-signed opaque continuation state.
			return true
		case "redacted_thinking":
			return true
		case "server_tool_use", "web_search_tool_result", "web_fetch_tool_result", "code_execution_tool_result",
			"bash_code_execution_tool_result", "text_editor_code_execution_tool_result", "mcp_tool_use",
			"mcp_tool_result", "container_upload":
			// These blocks are emitted by provider-side tools and can carry
			// provider-account IDs, fetched content, or container artifacts.
			return true
		default:
			// Anthropic beta headers can add new input block variants. Unknown
			// typed blocks fail closed until their ownership semantics have been
			// reviewed; tool_use.input remains deliberately opaque above.
			return true
		}
	}
	return false
}

func remoteURLReference(value any) bool {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed != "" && !strings.HasPrefix(strings.ToLower(trimmed), "data:")
	case map[string]any:
		if nested, ok := jsonObjectLookup(typed, "url"); ok {
			return remoteURLReference(nested)
		}
		return meaningfulJSONValue(typed)
	default:
		return meaningfulJSONValue(value)
	}
}

func unsafeContainerReference(value any) bool {
	if text, ok := value.(string); ok {
		trimmed := strings.TrimSpace(text)
		return trimmed != "" && !strings.EqualFold(trimmed, "auto")
	}
	if object, ok := value.(map[string]any); ok {
		if meaningfulJSONValue(jsonObjectValue(object, "id")) || meaningfulJSONValue(jsonObjectValue(object, "file_ids")) || meaningfulJSONValue(jsonObjectValue(object, "upload_ids")) || toolSkillsHaveProviderReferences(jsonObjectValue(object, "skills")) {
			return true
		}
		typeName, _ := jsonObjectValue(object, "type").(string)
		return typeName != "" && !strings.EqualFold(strings.TrimSpace(typeName), "auto")
	}
	return false
}

func objectType(object map[string]any) string {
	typeName, _ := jsonObjectValue(object, "type").(string)
	return strings.ToLower(strings.TrimSpace(typeName))
}

func jsonObjectValue(object map[string]any, key string) any {
	value, _ := jsonObjectLookup(object, key)
	return value
}

func jsonObjectLookup(object map[string]any, key string) (any, bool) {
	if value, ok := object[key]; ok {
		return value, true
	}
	for candidate, value := range object {
		if strings.EqualFold(candidate, key) {
			return value, true
		}
	}
	return nil, false
}

func meaningfulJSONValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func guardedProviderRouteMethod(requestPath string) (string, bool) {
	providerPath, ok := providerRoutePath(requestPath)
	if !ok {
		return "", false
	}
	switch providerPath {
	case "/v1/chat/completions", "/v1/embeddings", "/v1/messages", "/v1/responses":
		return http.MethodPost, true
	case "/v1/models":
		return http.MethodGet, true
	case "/v1/messages/count_tokens":
		return http.MethodPost, true
	default:
		return "", false
	}
}

func providerRoutePath(requestPath string) (string, bool) {
	trimmed := strings.TrimPrefix(requestPath, "/")
	providerName, providerPath, ok := strings.Cut(trimmed, "/")
	if !ok || strings.TrimSpace(providerName) == "" {
		return "", false
	}
	return "/" + providerPath, true
}

func writeMethodNotAllowed(w http.ResponseWriter, allowedMethod string) {
	w.Header().Set("Allow", allowedMethod)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
}

func writeProviderRouteNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "route_not_allowed"})
}

func writeProviderRequestError(w http.ResponseWriter, statusCode int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
