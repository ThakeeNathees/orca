package token

import "testing"

func TestTokenCreation(t *testing.T) {
	tok := Token{
		Type:    IDENT,
		Literal: "hello",
		Line:    1,
		Column:  5,
	}

	if tok.Type != IDENT {
		t.Errorf("expected type IDENT, got %s", tok.Type)
	}
	if tok.Literal != "hello" {
		t.Errorf("expected literal hello, got %s", tok.Literal)
	}
	if tok.Line != 1 || tok.Column != 5 {
		t.Errorf("expected position 1:5, got %d:%d", tok.Line, tok.Column)
	}
}

func TestLookupIdent(t *testing.T) {
	if LookupIdent("model") != MODEL {
		t.Errorf("expected MODEL for 'model'")
	}
	if LookupIdent("agent") != AGENT {
		t.Errorf("expected AGENT for 'agent'")
	}
	if LookupIdent("let") != LET {
		t.Errorf("expected LET for 'let'")
	}
	if LookupIdent("foobar") != IDENT {
		t.Errorf("expected IDENT for 'foobar'")
	}
}

func TestIsTokenBlockNameLet(t *testing.T) {
	if !IsTokenBlockName(LET) {
		t.Error("expected LET to be a block keyword")
	}
}

// TestBlockKindString verifies string representation for all BlockKind values.
func TestBlockKindString(t *testing.T) {
	tests := []struct {
		kind   BlockKind
		expect string
	}{
		{BlockModel, "model"},
		{BlockAgent, "agent"},
		{BlockTool, "tool"},
		{BlockTask, "task"},
		{BlockKnowledge, "knowledge"},
		{BlockWorkflow, "workflow"},
		{BlockTrigger, "trigger"},
		{BlockInput, "input"},
		{BlockSchema, "schema"},
		{BlockLet, "let"},
		{BlockKind(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expect, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expect {
				t.Errorf("BlockKind(%d).String() = %q, want %q", tt.kind, got, tt.expect)
			}
		})
	}
}

// TestTokenTypeToBlockKind verifies mapping from token types to block kinds.
func TestTokenTypeToBlockKind(t *testing.T) {
	tests := []struct {
		name     string
		tokType  TokenType
		expected BlockKind
		ok       bool
	}{
		{"MODEL", MODEL, BlockModel, true},
		{"AGENT", AGENT, BlockAgent, true},
		{"TOOL", TOOL, BlockTool, true},
		{"TASK", TASK, BlockTask, true},
		{"KNOWLEDGE", KNOWLEDGE, BlockKnowledge, true},
		{"WORKFLOW", WORKFLOW, BlockWorkflow, true},
		{"TRIGGER", TRIGGER, BlockTrigger, true},
		{"INPUT", INPUT, BlockInput, true},
		{"SCHEMA", SCHEMA, BlockSchema, true},
		{"LET", LET, BlockLet, true},
		{"IDENT not a block", IDENT, 0, false},
		{"STRING not a block", STRING, 0, false},
		{"EOF not a block", EOF, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, ok := TokenTypeToBlockKind(tt.tokType)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && kind != tt.expected {
				t.Errorf("kind = %v, want %v", kind, tt.expected)
			}
		})
	}
}

// TestBlockKindsSlice verifies that BlockKinds contains all block kinds
// and no primitives.
func TestBlockKindsSlice(t *testing.T) {
	if len(BlockKinds) != 10 {
		t.Errorf("len(BlockKinds) = %d, want 10", len(BlockKinds))
	}
	// All entries should be block kinds (String() != "unknown").
	for _, k := range BlockKinds {
		if k.String() == "unknown" {
			t.Errorf("BlockKinds contains unknown kind: %d", k)
		}
	}
}

// TestDescribe exercises Describe for all token types to verify human-readable descriptions.
// Coverage: exercises Describe for all token types
func TestDescribe(t *testing.T) {
	tests := []struct {
		name     string
		tokType  TokenType
		expected string
	}{
		{"ILLEGAL", ILLEGAL, "illegal character"},
		{"EOF", EOF, "end of file"},
		{"IDENT", IDENT, "identifier"},
		{"INT", INT, "integer"},
		{"FLOAT", FLOAT, "number"},
		{"STRING", STRING, "string"},
		{"RAWSTRING", RAWSTRING, "raw string"},
		{"TRUE", TRUE, "boolean"},
		{"FALSE", FALSE, "boolean"},
		{"NULL", NULL, "null"},
		{"ASSIGN", ASSIGN, "'='"},
		{"DOT", DOT, "'.'"},
		{"COMMA", COMMA, "','"},
		{"LBRACE", LBRACE, "'{'"},
		{"RBRACE", RBRACE, "'}'"},
		{"LBRACKET", LBRACKET, "'['"},
		{"RBRACKET", RBRACKET, "']'"},
		{"LPAREN", LPAREN, "'('"},
		{"RPAREN", RPAREN, "')'"},
		{"COLON", COLON, "':'"},
		{"PLUS", PLUS, "'+'"},
		{"MINUS", MINUS, "'-'"},
		{"STAR", STAR, "'*'"},
		{"SLASH", SLASH, "'/'"},
		{"ARROW", ARROW, "'->'"},
		{"PIPE", PIPE, "'|'"},
		{"AT", AT, "'@'"},
		{"MODEL keyword", MODEL, "'MODEL'"},
		{"AGENT keyword", AGENT, "'AGENT'"},
		{"TASK keyword", TASK, "'TASK'"},
		{"KNOWLEDGE keyword", KNOWLEDGE, "'KNOWLEDGE'"},
		{"TRIGGER keyword", TRIGGER, "'TRIGGER'"},
		{"WORKFLOW keyword", WORKFLOW, "'WORKFLOW'"},
		{"TOOL keyword", TOOL, "'TOOL'"},
		{"INPUT keyword", INPUT, "'INPUT'"},
		{"SCHEMA keyword", SCHEMA, "'SCHEMA'"},
		{"LET keyword", LET, "'LET'"},
		{"unknown token", TokenType("UNKNOWN"), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Describe(tt.tokType); got != tt.expected {
				t.Errorf("Describe(%s) = %q, want %q", tt.tokType, got, tt.expected)
			}
		})
	}
}

// TestIsIdentLike verifies that IDENT, NULL, and block keywords are ident-like,
// while other token types are not.
// Coverage: exercises IsIdentLike for all token types
func TestIsIdentLike(t *testing.T) {
	tests := []struct {
		name     string
		tokType  TokenType
		expected bool
	}{
		{"IDENT", IDENT, true},
		{"NULL", NULL, true},
		{"MODEL", MODEL, true},
		{"AGENT", AGENT, true},
		{"TOOL", TOOL, true},
		{"TASK", TASK, true},
		{"KNOWLEDGE", KNOWLEDGE, true},
		{"WORKFLOW", WORKFLOW, true},
		{"TRIGGER", TRIGGER, true},
		{"INPUT", INPUT, true},
		{"SCHEMA", SCHEMA, true},
		{"LET", LET, true},
		{"STRING not ident-like", STRING, false},
		{"INT not ident-like", INT, false},
		{"TRUE not ident-like", TRUE, false},
		{"FALSE not ident-like", FALSE, false},
		{"EOF not ident-like", EOF, false},
		{"LBRACE not ident-like", LBRACE, false},
		{"PLUS not ident-like", PLUS, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsIdentLike(tt.tokType); got != tt.expected {
				t.Errorf("IsIdentLike(%s) = %v, want %v", tt.tokType, got, tt.expected)
			}
		})
	}
}

// TestPrecedence verifies that operator tokens return the correct binding power
// and non-operator tokens return PrecLowest.
// Coverage: exercises Precedence for all token types
func TestPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		tokType  TokenType
		expected int
	}{
		{"ARROW", ARROW, PrecArrow},
		{"PIPE", PIPE, PrecPipe},
		{"PLUS", PLUS, PrecSum},
		{"MINUS", MINUS, PrecSum},
		{"STAR", STAR, PrecProduct},
		{"SLASH", SLASH, PrecProduct},
		{"DOT", DOT, PrecAccess},
		{"LBRACKET", LBRACKET, PrecAccess},
		{"LPAREN", LPAREN, PrecAccess},
		{"IDENT lowest", IDENT, PrecLowest},
		{"EOF lowest", EOF, PrecLowest},
		{"RBRACE lowest", RBRACE, PrecLowest},
		{"ASSIGN lowest", ASSIGN, PrecLowest},
		{"COMMA lowest", COMMA, PrecLowest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Precedence(tt.tokType); got != tt.expected {
				t.Errorf("Precedence(%s) = %d, want %d", tt.tokType, got, tt.expected)
			}
		})
	}
}

// TestIsTokenBlockNameAllKeywords verifies all block keywords return true.
func TestIsTokenBlockNameAllKeywords(t *testing.T) {
	blockTokens := []TokenType{MODEL, AGENT, TASK, KNOWLEDGE, TRIGGER, WORKFLOW, TOOL, INPUT, SCHEMA, LET}
	for _, tok := range blockTokens {
		if !IsTokenBlockName(tok) {
			t.Errorf("IsTokenBlockName(%s) = false, want true", tok)
		}
	}

	nonBlockTokens := []TokenType{IDENT, STRING, INT, FLOAT, TRUE, FALSE, NULL, EOF, LBRACE}
	for _, tok := range nonBlockTokens {
		if IsTokenBlockName(tok) {
			t.Errorf("IsTokenBlockName(%s) = true, want false", tok)
		}
	}
}
