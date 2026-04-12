package types

// The only core construct in the Orca's core is schema.
const (
	BlockKindSchema = "schema"

	BlockKindAny    = "any"
	BlockKindNull   = "null"
	BlockKindNumber = "number"
	BlockKindString = "string"

	BlockKindLet       = "let"
	BlockKindWorkflow  = "workflow"
	BlockKindTool      = "tool"
	BlockKindKnowledge = "knowledge"
	BlockKindModel     = "model"
	BlockKindAgent     = "agent"
	BlockKindCron      = "cron"
	BlockKindWebhook   = "webhook"
	BlockKindBranch    = "branch"

	AnnotationOnlyAssignments = "only_assignments"
	AnnotationWorkflowNode    = "workflow_node"
	AnnotationWorkflowChain   = "workflow_chain"
	AnnotationTriggerNode     = "trigger_node"

	LangTagMarkdown   = "md"
	LangTagPython     = "py"
	LangTagJavaScript = "js"
	LangTagJson       = "json"
	LangTagYaml       = "yaml"
)
