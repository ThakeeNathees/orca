from apscheduler.schedulers.blocking import BlockingScheduler
from langchain_openai import ChatOpenAI
from langchain_core.tools import tool
from langchain_core.messages import SystemMessage
from langgraph.graph import StateGraph, MessagesState
from tools.stocks import fetch as fetch_stocks_impl

gpt4 = ChatOpenAI(model="gpt-4o")

@tool
def fetch_stocks(ticker: str) -> str:
    """Fetch stock prices."""
    return fetch_stocks_impl(ticker)

gpt4_with_tools = gpt4.bind_tools([fetch_stocks])

def analyst(state: MessagesState):
    sys = "You summarise market movement."
    msgs = [SystemMessage(content=sys)] + state["messages"]
    return {"messages": [gpt4_with_tools.invoke(msgs)]}

graph = StateGraph(MessagesState)
graph.add_node("analyst", analyst)
graph.add_edge("__start__", "analyst")
graph.add_edge("analyst", "__end__")
app = graph.compile()

def run_morning_brief():
    app.invoke({"messages": []})

scheduler = BlockingScheduler()
scheduler.add_job(run_morning_brief, "cron",
                  day_of_week="mon-fri", hour=13, minute=30)
scheduler.start()
