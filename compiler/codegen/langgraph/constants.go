package langgraph

// orcaPrefix is the namespace prefix for all Orca runtime symbols in generated
// Python code. Block constructors become __orca_model(), __orca_agent(), etc.
const orcaPrefix = "__orca_"

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
)
