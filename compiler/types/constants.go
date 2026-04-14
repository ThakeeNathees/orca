package types

// The only core construct in the Orca's core is schema.
const (
	BlockKindSchema = "schema"

	BlockKindAny    = "any"
	BlockKindNull   = "null"
	BlockKindNumber = "number"
	BlockKindString = "string"

	BuiltinIdentifierTrue  = "true"
	BuiltinIdentifierFalse = "false"

	BlockKindLet       = "let"
	BlockKindWorkflow  = "workflow"
	BlockKindTool      = "tool"
	BlockKindKnowledge = "knowledge"
	BlockKindModel     = "model"
	BlockKindAgent     = "agent"
	BlockKindCron      = "cron"
	BlockKindWebhook   = "webhook"
	BlockKindBranch    = "branch"

	AnnotationWorkflowNode  = "workflow_node"
	AnnotationWorkflowChain = "workflow_chain"
	AnnotationTriggerNode   = "trigger_node"
	AnnotationStrictCheck   = "strict_check"

	LangTagMarkdown   = "md"
	LangTagPython     = "py"
	LangTagJavaScript = "js"
	LangTagJson       = "json"
	LangTagYaml       = "yaml"
)
