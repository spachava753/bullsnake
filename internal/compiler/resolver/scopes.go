package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

func (state *resolver) childScope(
	node compilerast.Node,
	purpose ScopePurpose,
	item int,
	kind ScopeKind,
	name string,
	privateName string,
	flags ScopeFlags,
) *Scope {
	if state.current.Kind == FunctionScope || state.current.Kind == TypeParametersScope || state.current.Flags&Nested != 0 {
		flags |= Nested
	}
	scope := &Scope{
		Kind:        kind,
		Name:        name,
		Range:       node.Span(),
		Parent:      state.current,
		PrivateName: privateName,
		Symbols:     make(map[string]*Symbol),
		Flags:       flags,
	}
	state.current.Children = append(state.current.Children, scope)
	state.table.scopes[scopeKey{node: node, purpose: purpose, item: item}] = scope
	return scope
}

func (state *resolver) markVisibleClassNeedsDict() {
	for scope := state.current; scope != nil; scope = scope.Parent {
		if scope.Kind == ClassScope {
			scope.Flags |= NeedsClassDict
			return
		}
	}
}

func (state *resolver) inScope(scope *Scope, collect func() error) error {
	parent := state.current
	parentControl := state.control
	parentComprehensionIterable := state.comprehensionIterable
	state.current = scope
	state.control = controlContext{}
	state.comprehensionIterable = 0
	err := collect()
	state.current = parent
	state.control = parentControl
	state.comprehensionIterable = parentComprehensionIterable
	return err
}
