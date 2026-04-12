package langgraph

// orcaPrefix is the namespace prefix for all Orca runtime symbols in generated
// Python code. Block constructors become __orca_model(), __orca_agent(), etc.
const orcaPrefix = "_orca__"

// Generated Python field and function names built from orcaPrefix.
// Centralised here so a prefix change propagates everywhere.
const (
	// orcaTriggerField is the state field that holds the trigger source name.
	orcaTriggerField = orcaPrefix + "trigger"

	// orcaPayloadField is the state field that holds the trigger payload.
	orcaPayloadField = orcaPrefix + "payload"

	// orcaGatherFunc is the runtime helper that collects predecessor outputs.
	orcaGatherFunc = orcaPrefix + "gather"

	// orcaInvokeAgentFunc is the runtime helper that invokes an agent node.
	orcaInvokeAgentFunc = orcaPrefix + "invoke_agent"

	// orcaInvokeToolFunc is the runtime helper that invokes a tool node.
	orcaInvokeToolFunc = orcaPrefix + "invoke_tool"

	// orcaBranchRouteKeyPrefix is the prefix for the state field that stores
	// a branch's computed route key. The full field name for a branch named
	// "foo" is orcaBranchRouteKeyPrefix + "foo" = "_orca__route__foo".
	// The branch node function writes the route key here; the branch router
	// function reads it to dispatch conditional edges.
	orcaBranchRouteKeyPrefix = orcaPrefix + "route__"
)

// orcaBranchRouteField returns the state field name that stores the route
// key produced by a branch's transform. See orcaBranchRouteKeyPrefix.
func orcaBranchRouteField(branchName string) string {
	return orcaBranchRouteKeyPrefix + branchName
}
