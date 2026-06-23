from typing import Optional
from pydantic import BaseModel
from crewai import Agent, Task, Crew, Process
from crewai.llm import LLM

class Ticket(BaseModel):
    id: str
    subject: str
    body: str
    priority: Optional[str] = None

class Resolution(BaseModel):
    ticket_id: str
    category: str
    reply: str

gpt4 = LLM(model="gpt-4o")

triager = Agent(
    role="Triager",
    goal="Classify tickets by category.",
    backstory="You triage customer support tickets.",
    llm=gpt4,
)

responder = Agent(
    role="Responder",
    goal="Draft a reply.",
    backstory="You write customer replies.",
    llm=gpt4,
)

qa = Agent(
    role="QA",
    goal="Check tone of reply.",
    backstory="You audit support replies.",
    llm=gpt4,
)

triage_task = Task(
    description="Triage ticket {ticket}.",
    expected_output="A Resolution object.",
    output_pydantic=Resolution,
    agent=triager,
)

reply_task = Task(
    description="Draft a reply using the triage result.",
    expected_output="A reply.",
    agent=responder,
    context=[triage_task],
)

qa_task = Task(
    description="Review the reply for tone.",
    expected_output="Final reply.",
    agent=qa,
    context=[reply_task],
)

crew = Crew(
    agents=[triager, responder, qa],
    tasks=[triage_task, reply_task, qa_task],
    process=Process.sequential,
)
