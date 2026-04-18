from crewai import Agent, Task, Crew, Process
from crewai.llm import LLM
from crewai_tools import SerperDevTool

claude = LLM(model="claude-opus-4.6")
search = SerperDevTool()

researcher = Agent(
    role="Researcher",
    goal="Gather current tech trends.",
    backstory="You research tech trends.",
    tools=[search],
    llm=claude,
)

writer = Agent(
    role="Writer",
    goal="Write a concise report.",
    backstory="You write concise reports.",
    llm=claude,
)

research_task = Task(
    description="Research {topic}.",
    expected_output="Notes.",
    agent=researcher,
)

write_task = Task(
    description="Turn notes into a report.",
    expected_output="Report.",
    agent=writer,
    context=[research_task],
)

crew = Crew(
    agents=[researcher, writer],
    tasks=[research_task, write_task],
    process=Process.sequential,
)
