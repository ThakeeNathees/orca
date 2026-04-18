from crewai import Agent, Task, Crew, Process
from crewai.llm import LLM

gpt4 = LLM(model="gpt-4o")

assistant = Agent(
    role="Assistant",
    goal="Answer the user's question.",
    backstory="You are a helpful assistant.",
    llm=gpt4,
)

task = Task(
    description="{question}",
    expected_output="A helpful answer.",
    agent=assistant,
)

crew = Crew(
    agents=[assistant],
    tasks=[task],
    process=Process.sequential,
)
