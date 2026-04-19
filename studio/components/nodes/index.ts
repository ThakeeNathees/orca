import type { NodeTypes } from "@xyflow/react";
import { BaseNode } from "./base-node";
import { BranchNode } from "./branch-node";

/**
 * Maps each Orca block kind to a React Flow custom node component.
 * All kinds share the same BaseNode renderer which reads the `kind`
 * from node data to decide colors, handles, and inline summary.
 */
export const nodeTypes: NodeTypes = {
  agent: BaseNode,
  web_search: BaseNode,
  code_exec: BaseNode,
  api_request: BaseNode,
  sql_query: BaseNode,
  knowledge: BaseNode,
  workflow: BaseNode,
  input: BaseNode,
  schema: BaseNode,
  cron: BaseNode,
  webhook: BaseNode,
  chat: BaseNode,
  custom_tool: BaseNode,
  branch: BranchNode,
};
