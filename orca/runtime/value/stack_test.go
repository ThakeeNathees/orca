package value

import "testing"

func TestStackLookup(t *testing.T) {
	xVal := NewNumber(1)
	yVal := NewNumber(2)

	root := NewStack(nil, map[string]*Value{"x": &xVal})
	child := NewStack(root, map[string]*Value{"y": &yVal})

	tests := []struct {
		name    string
		s       *Stack
		key     string
		wantNum float64
		wantOk  bool
	}{
		{name: "local_hit", s: child, key: "y", wantNum: 2, wantOk: true},
		{name: "parent_chain", s: child, key: "x", wantNum: 1, wantOk: true},
		{name: "missing", s: child, key: "z", wantOk: false},
		{name: "root_only", s: root, key: "x", wantNum: 1, wantOk: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := tt.s.Lookup(tt.key)
			if ok != tt.wantOk {
				t.Fatalf("Lookup(%q) ok=%v, want %v", tt.key, ok, tt.wantOk)
			}
			if !tt.wantOk {
				return
			}
			n, no := v.Number()
			if !no || n != tt.wantNum {
				t.Fatalf("number = (%v,%v), want %v", n, no, tt.wantNum)
			}
		})
	}
}
