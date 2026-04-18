from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage
from langgraph.graph import StateGraph, MessagesState

gpt4 = ChatOpenAI(model="gpt-4o")

def assistant(state: MessagesState):
    sys = "You are a helpful assistant."
    msgs = [SystemMessage(content=sys)] + state["messages"]
    return {"messages": [gpt4.invoke(msgs)]}

graph = StateGraph(MessagesState)
graph.add_node("assistant", assistant)
graph.add_edge("__start__", "assistant")
graph.add_edge("assistant", "__end__")
app = graph.compile()
