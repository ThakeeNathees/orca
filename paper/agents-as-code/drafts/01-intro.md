This paper presents Orca, a declarative, domain specific language tailored for for the orchestration of autonomous AI agents. 

Orca is a novel programming language developed as a more robust abstraction to languages such as Python /(cite) and libraries such as LangGraph /(cite), which form the modern programming stack when it comes to LLMs and agentic systems /(cite). It aims to address the limitations of the current imperative /(cite) approach by introducing an alternative paradigm where agent configurations, tool bindings and execution graphs are treated as first-class primitives and represented in a declarative manner. This shift eliminates a significant amount of boilerplate code cecessitated by the incumbent frameworks, and facilitates the lifting of dynamic misconfiguration errors into the static domain, allowing for compile-time validation of agent topologies and tool-binding constraints.

Orca adopts an HCL-inspired~\cite{terraform} declarative block syntax where developers simply describe the desired state of the system, and the Orca compiler translates
these declarations into fully functional Python/LangGraph code, performing
static analysis along the way.

This paper makes the following contributions:
\begin{enumerate}[leftmargin=*]
  \item A \textbf{declarative language design} for AI agent orchestration,
        with a uniform block syntax covering models, agents, tools, tasks,
        workflows, and triggers.
  \item A \textbf{four-stage compiler} (lexer, Pratt parser, semantic
        analyzer, code generator) implemented in Go, featuring error-tolerant
        parsing and a unified type system.
  \item A \textbf{static analysis framework} that catches reference errors,
        type mismatches, and schema violations at compile time, with
        diagnostic codes suitable for IDE integration.
  \item \textbf{Source-mapped code generation} that annotates every line of
        generated Python with its originating \texttt{.oc} source location,
        enabling DSL-level debugging.
\end{enumerate}