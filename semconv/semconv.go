// Package semconv defines OpenTelemetry GenAI semantic convention attribute keys
// used by go-ai-obs. These follow the OpenTelemetry GenAI Semantic Conventions
// (experimental, v1.28+).
//
// Reference: https://opentelemetry.io/docs/specs/semconv/gen-ai/
package semconv

// Operation names define the type of GenAI operation being performed.
const (
	OperationChat           = "chat"
	OperationTextCompletion = "text_completion"
	OperationEmbeddings     = "embeddings"
	OperationExecuteTool    = "execute_tool"
	OperationCreateAgent    = "create_agent"
	OperationInvokeAgent    = "invoke_agent"
)

// Span name templates.
const (
	SpanChat        = "chat %s"         // "chat gpt-4o"
	SpanEmbeddings  = "embeddings %s"   // "embeddings text-embedding-3-small"
	SpanExecuteTool = "execute_tool %s" // "execute_tool search_orders"
)

// GenAI common attributes.
const (
	AttrOperationName = "gen_ai.operation.name"
	AttrProviderName  = "gen_ai.system"
	AttrConversationID = "gen_ai.conversation.id"
)

// GenAI request attributes.
const (
	AttrRequestModel       = "gen_ai.request.model"
	AttrRequestMaxTokens   = "gen_ai.request.max_tokens"
	AttrRequestTemperature = "gen_ai.request.temperature"
	AttrRequestTopP        = "gen_ai.request.top_p"
	AttrRequestTopK        = "gen_ai.request.top_k"
	AttrRequestFrequencyPenalty = "gen_ai.request.frequency_penalty"
	AttrRequestPresencePenalty  = "gen_ai.request.presence_penalty"
	AttrRequestStopSequences    = "gen_ai.request.stop_sequences"
	AttrRequestSeed        = "gen_ai.request.seed"
)

// GenAI response attributes.
const (
	AttrResponseModel        = "gen_ai.response.model"
	AttrResponseID           = "gen_ai.response.id"
	AttrResponseFinishReasons = "gen_ai.response.finish_reasons"
)

// GenAI usage attributes.
const (
	AttrUsageInputTokens  = "gen_ai.usage.input_tokens"
	AttrUsageOutputTokens = "gen_ai.usage.output_tokens"
	AttrUsageTotalTokens  = "gen_ai.usage.total_tokens"
	AttrUsageCostDollars  = "gen_ai.usage.cost_dollars"
)

// GenAI agent attributes.
const (
	AttrAgentName        = "gen_ai.agent.name"
	AttrAgentID          = "gen_ai.agent.id"
	AttrAgentDescription = "gen_ai.agent.description"
	AttrAgentVersion     = "gen_ai.agent.version"
)

// GenAI tool attributes.
const (
	AttrToolName   = "gen_ai.tool.name"
	AttrToolType   = "gen_ai.tool.type"
	AttrToolCallID = "gen_ai.tool.call.id"
)

// GenAI message attributes (opt-in, off by default for PII safety).
const (
	AttrInputMessages  = "gen_ai.input.messages"
	AttrOutputMessages = "gen_ai.output.messages"
)

// Additional go-ai-obs custom attributes.
const (
	AttrDurationMS       = "gen_ai.duration_ms"
	AttrMessagesCount    = "gen_ai.request.messages_count"
	AttrHasSystemPrompt  = "gen_ai.request.has_system_prompt"
	AttrDeploymentEnv    = "deployment.environment"
	AttrServiceName      = "service.name"
)
