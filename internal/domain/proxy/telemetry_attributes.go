package proxy

import (
	"strings"

	aibtracing "github.com/coder/aibridge/tracing"
	"go.opentelemetry.io/otel/attribute"
)

const (
	openInferenceSpanKind          = "openinference.span.kind"
	openInferenceSpanKindLLM       = "LLM"
	openInferenceSpanKindEmbedding = "EMBEDDING"
	openInferenceInputValue        = "input.value"
	openInferenceInputMIMEType     = "input.mime_type"
	openInferenceLLMSystem         = "llm.system"
	openInferenceLLMProvider       = "llm.provider"
	openInferenceLLMModelName      = "llm.model_name"
	openInferencePromptTokens      = "llm.token_count.prompt"
	openInferenceCompletionTokens  = "llm.token_count.completion"
	openInferenceTotalTokens       = "llm.token_count.total"
	openInferenceCacheReadTokens   = "llm.token_count.prompt_details.cache_read"
	openInferenceCacheWriteTokens  = "llm.token_count.prompt_details.cache_write"
	openInferenceReasoningTokens   = "llm.token_count.completion_details.reasoning"

	genAIProviderName             = "gen_ai.provider.name"
	genAIRequestModel             = "gen_ai.request.model"
	genAIOperationName            = "gen_ai.operation.name"
	genAIRequestStream            = "gen_ai.request.stream"
	genAIConversationID           = "gen_ai.conversation.id"
	genAIInputTokens              = "gen_ai.usage.input_tokens"
	genAIOutputTokens             = "gen_ai.usage.output_tokens"
	genAICacheReadInputTokens     = "gen_ai.usage.cache_read.input_tokens"
	genAICacheCreationInputTokens = "gen_ai.usage.cache_creation.input_tokens"
	genAICacheWriteInputTokens    = "gen_ai.usage.cache_write.input_tokens"
	genAIReasoningOutputTokens    = "gen_ai.usage.reasoning.output_tokens"
	genAIOperationChat            = "chat"
	genAIOperationEmbeddings      = "embeddings"
	standardURLPath               = "url.path"
	standardUserID                = "user.id"
	standardErrorType             = "error.type"
	standardErrorMessage          = "error.message"
	toolInvocationErrorType       = "tool_invocation_error"
)

// interceptionTelemetryAttributes maps AIBridge interception metadata to both
// the existing OpenInference convention and the OpenTelemetry GenAI convention.
func interceptionTelemetryAttributes(provider, model string, bridgeAttrs []attribute.KeyValue) []attribute.KeyValue {
	operation, kind := genAIOperationChat, openInferenceSpanKindLLM
	if isEmbeddingInterception(bridgeAttrs) {
		operation, kind = genAIOperationEmbeddings, openInferenceSpanKindEmbedding
	}

	attrs := []attribute.KeyValue{
		attribute.String(openInferenceSpanKind, kind),
		attribute.String(openInferenceLLMSystem, provider),
		attribute.String(openInferenceLLMProvider, provider),
		attribute.String(openInferenceLLMModelName, model),
		attribute.String(genAIProviderName, provider),
		attribute.String(genAIRequestModel, model),
		attribute.String(genAIOperationName, operation),
	}
	for _, bridgeAttr := range bridgeAttrs {
		switch string(bridgeAttr.Key) {
		case aibtracing.RequestPath:
			attrs = append(attrs, attribute.String(standardURLPath, bridgeAttr.Value.AsString()))
		case aibtracing.InitiatorID:
			attrs = append(attrs, attribute.String(standardUserID, bridgeAttr.Value.AsString()))
		case aibtracing.Model:
			attrs = append(attrs, attribute.String(genAIRequestModel, bridgeAttr.Value.AsString()))
		case aibtracing.Streaming:
			if bridgeAttr.Value.AsBool() {
				attrs = append(attrs, attribute.Bool(genAIRequestStream, true))
			}
		}
	}
	return attrs
}

func isEmbeddingInterception(attrs []attribute.KeyValue) bool {
	for _, attr := range attrs {
		if string(attr.Key) == aibtracing.RequestPath && strings.HasSuffix(strings.TrimRight(attr.Value.AsString(), "/"), "/embeddings") {
			return true
		}
	}
	return false
}

func tokenTelemetryAttributes(input, output, cacheRead, cacheCreation, reasoning int64) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.Int64(openInferencePromptTokens, input),
		attribute.Int64(openInferenceCompletionTokens, output),
		attribute.Int64(openInferenceTotalTokens, input+output),
		attribute.Int64(openInferenceCacheReadTokens, cacheRead),
		attribute.Int64(openInferenceCacheWriteTokens, cacheCreation),
		attribute.Int64(genAIInputTokens, input),
		attribute.Int64(genAIOutputTokens, output),
		attribute.Int64(genAICacheReadInputTokens, cacheRead),
		attribute.Int64(genAICacheCreationInputTokens, cacheCreation),
		attribute.Int64(genAICacheWriteInputTokens, cacheCreation),
	}
	if reasoning > 0 {
		attrs = append(attrs,
			attribute.Int64(openInferenceReasoningTokens, reasoning),
			attribute.Int64(genAIReasoningOutputTokens, reasoning),
		)
	}
	return attrs
}
