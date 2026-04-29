package value

type Stack struct {
	parent *Stack
	values map[string]*Value
}

func NewStack(parent *Stack, values map[string]*Value) *Stack {
	return &Stack{
		parent: parent,
		values: values,
	}
}

func (s *Stack) Lookup(name string) (*Value, bool) {
	if value, ok := s.values[name]; ok {
		return value, true
	}
	if s.parent != nil {
		return s.parent.Lookup(name)
	}
	return nil, false
}
