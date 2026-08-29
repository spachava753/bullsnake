package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

func (state *resolver) collectTypeAlias(statement *compilerast.TypeAliasStmt) error {
	if _, err := state.bind(statement.Name, Assigned, statement.Span()); err != nil {
		return err
	}
	canSeeClass := state.current.Kind == ClassScope || state.current.Flags&CanSeeClassScope != 0
	if len(statement.TypeParameters) == 0 {
		return state.collectTypeAliasValue(statement, canSeeClass)
	}
	return state.inTypeParameters(
		statement, statement.Name, statement.TypeParameters,
		state.current.PrivateName, canSeeClass,
		func() error {
			return state.collectTypeAliasValue(statement, canSeeClass)
		},
	)
}

func (state *resolver) collectTypeAliasValue(statement *compilerast.TypeAliasStmt, canSeeClass bool) error {
	flags := ScopeFlags(0)
	if canSeeClass {
		flags |= CanSeeClassScope
		state.markVisibleClassNeedsDict()
	}
	scope := state.childScope(
		statement, TypeAliasValue, 0, TypeAliasScope, statement.Name,
		state.current.PrivateName, flags,
	)
	return state.inScope(scope, func() error {
		return state.collectExpr(statement.Value)
	})
}
