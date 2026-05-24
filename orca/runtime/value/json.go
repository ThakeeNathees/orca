package value

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// ValueFromJSON parses JSON into a runtime Value (scalars, arrays, objects).
// Empty or whitespace-only input yields KindNil.
func ValueFromJSON(data []byte) (Value, error) {
	trim := bytes.TrimSpace(data)
	if len(trim) == 0 {
		return NewNil(), nil
	}
	dec := json.NewDecoder(bytes.NewReader(trim))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return Value{}, err
	}
	var probe any
	switch err := dec.Decode(&probe); err {
	case io.EOF:
		return interfaceToValue(raw)
	case nil:
		return Value{}, fmt.Errorf("multiple JSON values in document")
	default:
		return Value{}, err
	}
}

func interfaceToValue(raw any) (Value, error) {
	switch x := raw.(type) {
	case nil:
		return NewNil(), nil
	case bool:
		return NewBool(x), nil
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return Value{}, err
		}
		return NewNumber(f), nil
	case float64:
		return NewNumber(x), nil
	case string:
		return NewString(x), nil
	case []any:
		out := make([]Value, len(x))
		for i := range x {
			v, err := interfaceToValue(x[i])
			if err != nil {
				return Value{}, err
			}
			out[i] = v
		}
		return NewList(out...), nil
	case map[string]any:
		m := NewMap()
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v, err := interfaceToValue(x[k])
			if err != nil {
				return Value{}, err
			}
			var ok bool
			m, ok = m.MapWith(NewString(k), v)
			if !ok {
				return Value{}, fmt.Errorf("json map merge failed for key %q", k)
			}
		}
		return m, nil
	default:
		return Value{}, fmt.Errorf("unsupported JSON type %T", raw)
	}
}
