from apscheduler.schedulers.blocking import BlockingScheduler
from crewai import Agent, Task, Crew, Process
from crewai.llm import LLM
from crewai.tools import tool
from tools.stocks import fetch as fetch_stocks_impl

gpt4 = LLM(model="gpt-4o")

@tool("fetch_stocks")
def fetch_stocks(ticker: str) -> str:
    """Fetch stock prices."""
    return fetch_stocks_impl(ticker)

analyst = Agent(
    role="Analyst",
    goal="Summarise market movement.",
    backstory="You summarise market movement.",
    tools=[fetch_stocks],
    llm=gpt4,
)

task = Task(
    description="Produce a morning brief.",
    expected_output="A summary.",
    agent=analyst,
)

crew = Crew(
    agents=[analyst],
    tasks=[task],
    process=Process.sequential,
)

def run_morning_brief():
    crew.kickoff()

scheduler = BlockingScheduler()
scheduler.add_job(run_morning_brief, "cron",
                  day_of_week="mon-fri", hour=13, minute=30)
scheduler.start()
