package value

import (
	"github.com/thakee/orca/orca/compiler/analyzer"
	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/types"
)

// Kind classifies a runtime Value. Only the payload fields matching Kind are valid.
type Kind int

const (
	// KindNil is the null value.
	KindNil Kind = iota
	KindString
	KindNumber
	KindBool
	KindList
	KindMap
	KindBlock
	KindLambda
)

// Value is the runtime result of an expression. Instances are immutable: list and map
// updates return new Values with copied backing slices (structural sharing may come later).
type Value struct {
	kind      Kind
	str       string
	num       float64
	b         bool
	list      []Value
	keys      []Value
	vals      []Value
	blockKind string
	lambda    *ast.Lambda
	lambdaEnv *types.SymbolTable
}

// Kind returns the discriminant of v.
func (v Value) Kind() Kind {
	return v.kind
}

// NewNil returns the null value.
func NewNil() Value {
	return Value{kind: KindNil}
}

// NewString returns a string value.
func NewString(s string) Value {
	return Value{kind: KindString, str: s}
}

// NewNumber returns a numeric value.
func NewNumber(n float64) Value {
	return Value{kind: KindNumber, num: n}
}

// NewBool returns a boolean value.
func NewBool(b bool) Value {
	return Value{kind: KindBool, b: b}
}

// NewList returns a list value containing a defensive copy of elems.
func NewList(elems ...Value) Value {
	if len(elems) == 0 {
		return Value{kind: KindList, list: []Value{}}
	}
	out := make([]Value, len(elems))
	copy(out, elems)
	return Value{kind: KindList, list: out}
}

// NewMap returns an empty ordered map (parallel keys and values).
func NewMap() Value {
	return Value{kind: KindMap, keys: []Value{}, vals: []Value{}}
}

// NewBlock returns an empty block-shaped value with the given block kind name (e.g. "model").
func NewBlock(blockKind string) Value {
	return Value{kind: KindBlock, keys: []Value{}, vals: []Value{}, blockKind: blockKind}
}

// NewLambda returns a lambda value. env may be nil if no symbol environment is available yet.
func NewLambda(l *ast.Lambda, env *types.SymbolTable) Value {
	return Value{kind: KindLambda, lambda: l, lambdaEnv: env}
}

// String returns the string payload if Kind is KindString.
func (v Value) String() (string, bool) {
	if v.kind != KindString {
		return "", false
	}
	return v.str, true
}

// Number returns the numeric payload if Kind is KindNumber.
func (v Value) Number() (float64, bool) {
	if v.kind != KindNumber {
		return 0, false
	}
	return v.num, true
}

// Bool returns the boolean payload if Kind is KindBool.
func (v Value) Bool() (bool, bool) {
	if v.kind != KindBool {
		return false, false
	}
	return v.b, true
}

// List returns a copy of the list elements if Kind is KindList.
func (v Value) List() ([]Value, bool) {
	if v.kind != KindList {
		return nil, false
	}
	out := make([]Value, len(v.list))
	copy(out, v.list)
	return out, true
}

// KeysValues returns copies of parallel keys and values if Kind is KindMap or KindBlock.
func (v Value) KeysValues() (keys []Value, vals []Value, ok bool) {
	if v.kind != KindMap && v.kind != KindBlock {
		return nil, nil, false
	}
	keys = make([]Value, len(v.keys))
	vals = make([]Value, len(v.vals))
	copy(keys, v.keys)
	copy(vals, v.vals)
	return keys, vals, true
}

// BlockKind returns the block kind name if Kind is KindBlock.
func (v Value) BlockKind() (string, bool) {
	if v.kind != KindBlock {
		return "", false
	}
	return v.blockKind, true
}

// IsWebhookNode reports whether v is a KindBlock value whose schema kind is webhook.
func (v *Value) IsWebhookNode() bool {
	if v == nil {
		return false
	}
	bk, ok := v.BlockKind()
	return ok && bk == types.BlockKindWebhook
}

// LambdaParts returns the lambda AST and optional symbol environment if Kind is KindLambda.
func (v Value) LambdaParts() (*ast.Lambda, *types.SymbolTable, bool) {
	if v.kind != KindLambda {
		return nil, nil, false
	}
	return v.lambda, v.lambdaEnv, true
}

// Append returns a new list value with elem appended. The receiver must be KindList.
func (v Value) Append(elem Value) (Value, bool) {
	if v.kind != KindList {
		return Value{}, false
	}
	out := make([]Value, len(v.list)+1)
	copy(out, v.list)
	out[len(v.list)] = elem
	return Value{kind: KindList, list: out}, true
}

// MapWith returns a new map or block value: if key matches an existing entry (structural
// equality), the value is replaced; otherwise the pair is appended. The receiver must be
// KindMap or KindBlock.
func (v Value) MapWith(key, val Value) (Value, bool) {
	if v.kind != KindMap && v.kind != KindBlock {
		return Value{}, false
	}
	nk := make([]Value, len(v.keys))
	nv := make([]Value, len(v.vals))
	copy(nk, v.keys)
	copy(nv, v.vals)
	for i := range nk {
		if equalValue(nk[i], key) {
			nv[i] = val
			return Value{kind: v.kind, keys: nk, vals: nv, blockKind: v.blockKind}, true
		}
	}
	nk = append(nk, key)
	nv = append(nv, val)
	return Value{kind: v.kind, keys: nk, vals: nv, blockKind: v.blockKind}, true
}

// Merge combines prior output a with newer output b into a new immutable Value.
//
// Semantics:
//   - If a is KindNil (e.g. no prior node data), returns b.
//   - If b is KindNil, returns a.
//   - If both are KindMap, entries are merged: keys equal under equalValue have values
//     merged recursively; keys only present in b are appended (new parallel slices).
//   - If both are KindBlock with the same blockKind string, the same map-merge applies.
//   - Lists use replace semantics (same as LangGraph-style snapshots): when both sides are
//     KindList, b replaces a entirely; nested lists merge only via map key recursion below.
//   - Otherwise b replaces a (scalars, lambdas, KindList pairs, kind mismatches,
//     Map mixed with Block, or blocks with different blockKind).
func Merge(a, b Value) Value {
	if a.kind == KindNil {
		return b
	}
	if b.kind == KindNil {
		return a
	}
	if a.kind == KindMap && b.kind == KindMap {
		return mergeMapLike(a, b, KindMap, "")
	}
	if a.kind == KindBlock && b.kind == KindBlock && a.blockKind == b.blockKind {
		return mergeMapLike(a, b, KindBlock, a.blockKind)
	}
	return b
}

// mergeMapLike merges two map-like values (same Kind and, for blocks, same blockKind).
// Caller must ensure KindMap+KindMap or matching KindBlock pair.
func mergeMapLike(a, b Value, kind Kind, blockKind string) Value {
	nk := make([]Value, len(a.keys))
	nv := make([]Value, len(a.vals))
	copy(nk, a.keys)
	copy(nv, a.vals)
	for j := range b.keys {
		found := false
		for i := range nk {
			if equalValue(nk[i], b.keys[j]) {
				nv[i] = Merge(nv[i], b.vals[j])
				found = true
				break
			}
		}
		if !found {
			nk = append(nk, b.keys[j])
			nv = append(nv, b.vals[j])
		}
	}
	return Value{kind: kind, keys: nk, vals: nv, blockKind: blockKind}
}

// ValueFromConst builds a runtime Value from a compile-time folded constant.
// Returns ok false when the fold is unknown, partial, or not fully representable (including
// failed recursive conversions). For ConstLambda, lambdaSymtab is stored as the lambda
// environment; it is ignored for other kinds. cv.Lambda must be non-nil for ConstLambda.
func ValueFromConst(cv analyzer.ConstValue, lambdaSymtab *types.SymbolTable) (Value, bool) {
	if cv.Kind == analyzer.ConstUnknown || cv.Partial {
		return Value{}, false
	}
	switch cv.Kind {
	case analyzer.ConstString:
		return Value{kind: KindString, str: cv.Str}, true
	case analyzer.ConstNumber:
		return Value{kind: KindNumber, num: cv.Number}, true
	case analyzer.ConstBool:
		return Value{kind: KindBool, b: cv.Bool}, true
	case analyzer.ConstNull:
		return Value{kind: KindNil}, true
	case analyzer.ConstList:
		out := make([]Value, len(cv.List))
		for i := range cv.List {
			ev, ok := ValueFromConst(cv.List[i], lambdaSymtab)
			if !ok {
				return Value{}, false
			}
			out[i] = ev
		}
		return Value{kind: KindList, list: out}, true
	case analyzer.ConstMap:
		return valueFromConstPairs(cv.Keys, cv.Values, KindMap, "", lambdaSymtab)
	case analyzer.ConstBlock:
		return valueFromConstPairs(cv.Keys, cv.Values, KindBlock, cv.BlockKind, lambdaSymtab)
	case analyzer.ConstLambda:
		if cv.Lambda == nil {
			return Value{}, false
		}
		return Value{
			kind:      KindLambda,
			lambda:    cv.Lambda,
			lambdaEnv: lambdaSymtab,
		}, true
	default:
		return Value{}, false
	}
}

func valueFromConstPairs(
	keys []analyzer.ConstValue,
	vals []analyzer.ConstValue,
	kind Kind,
	blockKind string,
	sym *types.SymbolTable,
) (Value, bool) {
	if len(keys) != len(vals) {
		return Value{}, false
	}
	k := make([]Value, len(keys))
	v := make([]Value, len(vals))
	for i := range keys {
		var ok bool
		k[i], ok = ValueFromConst(keys[i], sym)
		if !ok {
			return Value{}, false
		}
		v[i], ok = ValueFromConst(vals[i], sym)
		if !ok {
			return Value{}, false
		}
	}
	return Value{kind: kind, keys: k, vals: v, blockKind: blockKind}, true
}

func equalValue(a, b Value) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case KindNil:
		return true
	case KindString:
		return a.str == b.str
	case KindNumber:
		return a.num == b.num
	case KindBool:
		return a.b == b.b
	case KindList:
		if len(a.list) != len(b.list) {
			return false
		}
		for i := range a.list {
			if !equalValue(a.list[i], b.list[i]) {
				return false
			}
		}
		return true
	case KindMap, KindBlock:
		if len(a.keys) != len(a.vals) || len(b.keys) != len(b.vals) {
			return false
		}
		if len(a.keys) != len(b.keys) || a.blockKind != b.blockKind {
			return false
		}
		for i := range a.keys {
			if !equalValue(a.keys[i], b.keys[i]) || !equalValue(a.vals[i], b.vals[i]) {
				return false
			}
		}
		return true
	case KindLambda:
		return a.lambda == b.lambda && a.lambdaEnv == b.lambdaEnv
	default:
		return false
	}
}
