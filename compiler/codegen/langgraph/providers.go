// Package langgraph implements Python/LangGraph code generation from the IR.
package langgraph

// providerImport maps a provider name to its LangChain import statement.
var providerImport = map[string]string{
	"openai":    "from langchain_openai import ChatOpenAI",
	"anthropic": "from langchain_anthropic import ChatAnthropic",
	"google":    "from langchain_google_genai import ChatGoogleGenerativeAI",
}

// providerClass maps a provider name to its LangChain class name.
var providerClass = map[string]string{
	"openai":    "ChatOpenAI",
	"anthropic": "ChatAnthropic",
	"google":    "ChatGoogleGenerativeAI",
}

// providerDep maps provider names to pip package names.
var providerDep = map[string]string{
	"openai":    "langchain-openai",
	"anthropic": "langchain-anthropic",
	"google":    "langchain-google-genai",
}
