package analyzer

const (
	BlockKindInput     = "input"
	BlockKindLet       = "let"
	BlockKindWorkflow  = "workflow"
	BlockKindTool      = "tool"
	BlockKindKnowledge = "knowledge"
	BlockKindModel     = "model"
	BlockKindAgent     = "agent"
	BlockKindCron      = "cron"
	BlockKindWebhook   = "webhook"

	AnnotationOnlyAssignments = "only_assignments"
	AnnotationWorkflowNode    = "workflow_node"
	AnnotationTriggerNode     = "trigger_node"

	LangTagMarkdown   = "md"
	LangTagPython     = "py"
	LangTagJavaScript = "js"
	LangTagJson       = "json"
	LangTagYaml       = "yaml"
)
