from langchain_anthropic import ChatAnthropic
from langchain_community.tools import TavilySearchResults
from langchain_core.messages import SystemMessage
from langgraph.graph import StateGraph, MessagesState

claude = ChatAnthropic(model="claude-opus-4.6")
search = TavilySearchResults()
claude_with_tools = claude.bind_tools([search])

def researcher(state: MessagesState):
    sys = "You research tech trends."
    msgs = [SystemMessage(content=sys)] + state["messages"]
    return {"messages": [claude_with_tools.invoke(msgs)]}

def writer(state: MessagesState):
    sys = "You write concise reports."
    msgs = [SystemMessage(content=sys)] + state["messages"]
    return {"messages": [claude.invoke(msgs)]}

graph = StateGraph(MessagesState)
graph.add_node("researcher", researcher)
graph.add_node("writer", writer)
graph.add_edge("__start__", "researcher")
graph.add_edge("researcher", "writer")
graph.add_edge("writer", "__end__")
app = graph.compile()
