from typing import Optional, TypedDict
from pydantic import BaseModel
from langchain_openai import ChatOpenAI
from langchain_core.messages import SystemMessage
from langgraph.graph import StateGraph

class Ticket(BaseModel):
    id: str
    subject: str
    body: str
    priority: Optional[str] = None

class Resolution(BaseModel):
    ticket_id: str
    category: str
    reply: str

class SupportState(TypedDict):
    ticket: Ticket
    resolution: Optional[Resolution]
    messages: list

gpt4 = ChatOpenAI(model="gpt-4o")
structured = gpt4.with_structured_output(Resolution)

def triager(state: SupportState):
    sys = "Classify tickets by category."
    resp = structured.invoke(
        [SystemMessage(content=sys), str(state["ticket"])]
    )
    return {"resolution": resp}

def responder(state: SupportState):
    sys = "Draft a reply using the triage result."
    msg = gpt4.invoke(
        [SystemMessage(content=sys), str(state["resolution"])]
    )
    return {"messages": state["messages"] + [msg]}

def qa(state: SupportState):
    sys = "Check the reply for tone."
    msg = gpt4.invoke(
        [SystemMessage(content=sys)] + state["messages"]
    )
    return {"messages": state["messages"] + [msg]}

graph = StateGraph(SupportState)
graph.add_node("triager", triager)
graph.add_node("responder", responder)
graph.add_node("qa", qa)
graph.add_edge("__start__", "triager")
graph.add_edge("triager", "responder")
graph.add_edge("responder", "qa")
graph.add_edge("qa", "__end__")
app = graph.compile()
