package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

// collectComprehension evaluates the first iterable outside, then resolves the
// targets, filters, remaining clauses, and result in a new function-like scope.
func (state *resolver) collectComprehension(
	node compilerast.Expr,
	name string,
	kind ScopeFlags,
	clauses []compilerast.Comprehension,
	key compilerast.Expr,
	value compilerast.Expr,
) error {
	if len(clauses) == 0 {
		return unsupportedNode(node)
	}

	state.comprehensionIterable++
	err := state.collectExpr(clauses[0].Iterable)
	state.comprehensionIterable--
	if err != nil {
		return err
	}

	flags := kind
	if kind == GeneratorExpression {
		flags |= Generator
	}
	for _, clause := range clauses {
		if clause.Async {
			flags |= Coroutine
		}
	}
	comprehensionScope := state.childScope(
		node, ComprehensionBody, 0, FunctionScope, name,
		state.current.PrivateName, flags,
	)
	err = state.inScope(comprehensionScope, func() error {
		if err := state.collectComprehensionTarget(clauses[0].Target); err != nil {
			return err
		}
		for _, condition := range clauses[0].Conditions {
			if err := state.collectExpr(condition); err != nil {
				return err
			}
		}
		for _, clause := range clauses[1:] {
			state.comprehensionIterable++
			err := state.collectExpr(clause.Iterable)
			state.comprehensionIterable--
			if err != nil {
				return err
			}
			if err := state.collectComprehensionTarget(clause.Target); err != nil {
				return err
			}
			for _, condition := range clause.Conditions {
				if err := state.collectExpr(condition); err != nil {
					return err
				}
			}
		}
		if err := state.collectExpr(value); err != nil {
			return err
		}
		return state.collectExpr(key)
	})
	if err != nil {
		return err
	}
	if comprehensionScope.Flags&Coroutine != 0 && kind != GeneratorExpression {
		if isComprehension(state.current) {
			state.current.Flags |= Coroutine
		} else if state.current.Kind != FunctionScope || state.current.Flags&AsyncFunction == 0 {
			return state.syntaxError(
				node.Span(),
				"asynchronous comprehension outside of an asynchronous function",
			)
		}
	}
	return nil
}

// collectComprehensionTarget marks bound names as iteration variables while
// recursively visiting destructuring and attribute or subscript targets.
func (state *resolver) collectComprehensionTarget(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.Name:
		name := manglePrivate(state.current.PrivateName, expression.ID)
		if _, redirected := state.current.redirectedBindings[name]; redirected {
			return state.syntaxError(
				expression.Span(),
				"comprehension inner loop cannot rebind assignment expression target %q",
				name,
			)
		}
		_, err := state.bind(expression.ID, Assigned|ComprehensionIterator, expression.Span())
		return err
	case *compilerast.AttributeExpr, *compilerast.SubscriptExpr:
		return state.collectExpr(expression)
	case *compilerast.TupleExpr:
		for _, element := range expression.Elements {
			if err := state.collectComprehensionTarget(element); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.ListExpr:
		for _, element := range expression.Elements {
			if err := state.collectComprehensionTarget(element); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.StarredExpr:
		return state.collectComprehensionTarget(expression.Value)
	default:
		return unsupportedNode(expression)
	}
}

// collectNamedExpression resolves ordinary targets locally and redirects a
// comprehension target to its nearest enclosing function or module.
func (state *resolver) collectNamedExpression(expression *compilerast.NamedExpr) error {
	switch state.current.Kind {
	case AnnotationScope:
		return state.syntaxError(expression.Span(), "named expression cannot be used within an annotation")
	case TypeAliasScope:
		return state.syntaxError(expression.Span(), "named expression cannot be used within a type alias")
	case TypeVariableScope:
		return state.syntaxError(expression.Span(), "named expression cannot be used within a TypeVar bound")
	case TypeParametersScope:
		return state.syntaxError(expression.Span(), "named expression cannot be used within the definition of a generic")
	}
	if state.comprehensionIterable != 0 {
		return state.syntaxError(
			expression.Span(),
			"assignment expression cannot be used in a comprehension iterable expression",
		)
	}
	if err := state.collectExpr(expression.Value); err != nil {
		return err
	}
	if !isComprehension(state.current) {
		return state.collectExpr(expression.Target)
	}

	target, ok := expression.Target.(*compilerast.Name)
	if !ok {
		return unsupportedNode(expression.Target)
	}
	name := manglePrivate(state.current.PrivateName, target.ID)
	for scope := state.current; isComprehension(scope); scope = scope.Parent {
		if symbol := scope.Symbols[name]; symbol != nil && symbol.Flags&ComprehensionIterator != 0 {
			return state.syntaxError(
				target.Span(),
				"assignment expression cannot rebind comprehension iteration variable %q",
				name,
			)
		}
	}

	owner := state.current.Parent
	for isComprehension(owner) {
		owner = owner.Parent
	}
	if owner == nil {
		return unsupportedNode(expression)
	}
	switch owner.Kind {
	case AnnotationScope:
		return state.syntaxError(
			target.Span(),
			"named expression cannot be used within an annotation",
		)
	case ClassScope:
		return state.syntaxError(
			target.Span(),
			"assignment expression within a comprehension cannot be used in a class body",
		)
	case TypeAliasScope:
		return state.syntaxError(
			target.Span(),
			"assignment expression within a comprehension cannot be used in a type alias",
		)
	case TypeVariableScope:
		return state.syntaxError(
			target.Span(),
			"assignment expression within a comprehension cannot be used in a TypeVar bound",
		)
	case TypeParametersScope:
		return state.syntaxError(
			target.Span(),
			"assignment expression within a comprehension cannot be used within the definition of a generic",
		)
	}

	current := state.current
	state.current = owner
	ownerName, err := state.bind(target.ID, Assigned, target.Span())
	state.current = current
	if err != nil {
		return err
	}
	ownerSymbol := owner.Symbols[ownerName]

	name, err = state.bind(target.ID, Assigned, target.Span())
	if err != nil {
		return err
	}
	if state.current.redirectedBindings == nil {
		state.current.redirectedBindings = make(map[string]NameScope)
	}
	if owner.Kind == ModuleScope || ownerSymbol.Flags&GlobalDeclaration != 0 {
		state.current.redirectedBindings[name] = GlobalExplicit
	} else {
		state.current.redirectedBindings[name] = Free
	}
	return nil
}

func isComprehension(scope *Scope) bool {
	if scope == nil {
		return false
	}
	return scope.Flags&(ListComprehension|SetComprehension|DictComprehension|GeneratorExpression) != 0
}
