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

	// Graph terminal node names. These are virtual nodes representing the
	// start and end of a workflow graph — not user-defined blocks.
	NodeSTART = "START"
	NodeEND   = "END"

	// Field name "nodes" in the workflow block.
	NodesField = "nodes"

	// Branch schema field names (see compiler/types/bootstrap.orca). If the
	// schema renames these fields, update here too. Exported because the
	// analyzer also looks up these fields when validating route values.
	BranchFieldTransform = "transform"
	BranchFieldRoute     = "route"

	// BranchRouteKeyDefault is the route key used as the fallback target
	// when a branch's transform produces a key not in the explicit route
	// map. Codegen auto-injects {BranchRouteKeyDefault: END} if the user
	// did not provide a "default" entry, so LangGraph never receives an
	// unknown key at runtime.
	BranchRouteKeyDefault = "default"

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
