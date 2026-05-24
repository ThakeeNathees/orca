package agentgraph

import "testing"

func TestNormalizeWebhookPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty_defaults_slash", in: "", want: "/"},
		{name: "adds_leading_slash", in: "hooks/in", want: "/hooks/in"},
		{name: "preserves_absolute", in: "/hooks/in", want: "/hooks/in"},
		{name: "trims_space", in: "  /x  ", want: "/x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeWebhookPath(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeWebhookPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
