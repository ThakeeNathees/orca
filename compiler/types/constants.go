package types

// The only core construct in the Orca's core is schema.
const (
	BlockKindSchema = "schema"

	BlockKindAny       = "any"
	BlockKindNull      = "null"
	BlockKindNulltype  = "nulltype"
	BlockKindNumber    = "number"
	BlockKindString    = "string"
	BlockKindList      = "list"
	BlockKindMap       = "map"
	BlockKindCallable  = "callable"
	BlockKindAnnotated = "annotated"

	BuiltinIdentifierTrue  = "true"
	BuiltinIdentifierFalse = "false"

	BlockKindWorkflow = "workflow"
	BlockKindTool     = "tool"
	BlockKindModel    = "model"
	BlockKindAgent    = "agent"
	BlockKindCron     = "cron"
	BlockKindWebhook  = "webhook"
	BlockKindBranch   = "branch"

	AnnotationSuppress      = "suppress"
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
