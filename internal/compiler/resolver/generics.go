package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

// inTypeParameters creates the generic scope, records parameters and their
// bound or default children, then collects the definition below it.
func (state *resolver) inTypeParameters(
	owner compilerast.Node,
	name string,
	parameters []compilerast.TypeParameter,
	privateName string,
	canSeeClass bool,
	body func() error,
) error {
	flags := ScopeFlags(0)
	if canSeeClass {
		flags |= CanSeeClassScope
	}
	typeScope := state.childScope(
		owner, TypeParameters, 0, TypeParametersScope, name, privateName, flags,
	)
	return state.inScope(typeScope, func() error {
		for index, parameter := range parameters {
			mangled := manglePrivate(state.current.PrivateName, parameter.Name)
			if symbol := state.current.Symbols[mangled]; symbol != nil && symbol.Flags&TypeParameter != 0 {
				return state.syntaxError(parameter.Range, "duplicate type parameter %q", mangled)
			}
			if mangled == "__classdict__" {
				return state.syntaxError(parameter.Range, "reserved name %q cannot be used for type parameter", mangled)
			}
			if _, err := state.bind(parameter.Name, TypeParameter, parameter.Range); err != nil {
				return err
			}
			if err := state.collectTypeVariablePart(owner, parameter, index, TypeVariableBound, parameter.Bound); err != nil {
				return err
			}
			if err := state.collectTypeVariablePart(owner, parameter, index, TypeVariableDefault, parameter.Default); err != nil {
				return err
			}
		}
		return body()
	})
}

func (state *resolver) collectTypeVariablePart(
	owner compilerast.Node,
	parameter compilerast.TypeParameter,
	index int,
	purpose ScopePurpose,
	expression compilerast.Expr,
) error {
	if expression == nil {
		return nil
	}
	flags := ScopeFlags(0)
	if state.current.Flags&CanSeeClassScope != 0 {
		flags |= CanSeeClassScope
	}
	scope := state.childScope(
		owner, purpose, index, TypeVariableScope, parameter.Name,
		state.current.PrivateName, flags,
	)
	scope.Range = parameter.Range
	return state.inScope(scope, func() error {
		return state.collectExpr(expression)
	})
}
