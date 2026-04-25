package types

import "github.com/thakee/orca/orca/compiler/ast"

// HasAnnotation reports whether annotations contains a decorator with the
// given name (the identifier after @, without the @ prefix). Lives in the
// types package because annotation lookup is part of the type system —
// the helper package must not depend on any other compiler module.
func HasAnnotation(annotations []*ast.Annotation, name string) bool {
	for _, ann := range annotations {
		if ann != nil && ann.Name == name {
			return true
		}
	}
	return false
}

// IsAnnotated reports whether a type's underlying block (or its parent
// schema) carries the given annotation.
//
// Walks at most two schema hops:
//
//  1. typ.Block.Annotations — annotations on the block itself
//  2. typ.Block.Schema.Annotations — annotations on the parent schema
//     (the bootstrap schema for user instances like `tool foo {}`)
//
// For inline BlockExpressions, typ.Block may be nil because blockExprType
// returns BlockRef(kind, nil) without resolving the schema pointer; in that
// case the function returns false without crashing. Callers that need to
// handle that path can use IsBlockKind first or look up the kind in the
// symbol table themselves.
//
// IsAnnotated takes the annotation name as data, so the same helper covers
// every category check (workflow_node, trigger_node, only_assignments, ...).
// No type-specific predicates needed.
func IsAnnotated(typ Type, annotationName string) bool {
	if typ.Kind != BlockRef || typ.Block == nil {
		return false
	}
	if HasAnnotation(typ.Block.Annotations, annotationName) {
		return true
	}
	if typ.Block.Schema != nil && HasAnnotation(typ.Block.Schema.Annotations, annotationName) {
		return true
	}
	return false
}

// IsBlockKind reports whether a type resolves to a block of the given kind.
// Walks the same two-hop chain as IsAnnotated:
//
//  1. typ.BlockName matches kind
//  2. typ.Block.Schema.BlockName matches kind (for user instances whose
//     immediate type is the instance schema, with the kind name on the
//     parent schema)
//
// Used by call sites that need to check the structural kind of a block —
// e.g. "is this a branch?" — without inspecting annotations.
func IsBlockKind(typ Type, kind string) bool {
	if typ.Kind != BlockRef {
		return false
	}
	if typ.BlockName == kind {
		return true
	}
	if typ.Block != nil && typ.Block.Schema != nil && typ.Block.Schema.BlockName == kind {
		return true
	}
	return false
}
