// Package langgraph implements Python/LangGraph code generation from the IR.
package langgraph

// providerInfo holds LangChain metadata for a model provider.
type providerInfo struct {
	Import string // e.g. "from langchain_openai import ChatOpenAI"
	Class  string // e.g. "ChatOpenAI"
	Dep    string // pip package name, e.g. "langchain-openai"
}

// providers maps provider names to their LangChain metadata.
var providerRegistry = map[string]providerInfo{
	"openai":    {Import: "from langchain_openai import ChatOpenAI", Class: "ChatOpenAI", Dep: "langchain-openai"},
	"anthropic": {Import: "from langchain_anthropic import ChatAnthropic", Class: "ChatAnthropic", Dep: "langchain-anthropic"},
	"google":    {Import: "from langchain_google_genai import ChatGoogleGenerativeAI", Class: "ChatGoogleGenerativeAI", Dep: "langchain-google-genai"},
}
