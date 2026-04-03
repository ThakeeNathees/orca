/**
 * Default buffer for the Studio code pane until graph ↔ .oc sync exists.
 * Kept in sync conceptually with ../../orca/main.oc (sample workflow).
 */
export const SAMPLE_ORCA_SOURCE = `

// The model block is used to configure the model to use for the agent.
model gpt4o {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.7
}

// Note that this is a dummy block and the real implementation is still WIP.
memory session_memory {
  id        = "session_store"
  description = "Conversation and tool-call history"
}

// The agent block is used to define the agent.
agent researcher {
  model   = gpt4o
  persona = "Find and summarize information."
  tools   = [
    builtin.web_search,
    builtin.code_interpreter,
  ]
  memory  = session_memory
}

`;
