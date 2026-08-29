package resolver

import "sort"

type nameSet map[string]struct{}

// analyzeScope classifies one completed scope, passes visible bindings to its
// children, and returns closure names requested from its parent.
func (state *resolver) analyzeScope(
	scope *Scope,
	enclosingBindings nameSet,
	enclosingGlobals nameSet,
	typeParameters nameSet,
	visibleClass *Scope,
) (nameSet, error) {
	localBindings := make(nameSet)
	currentGlobals := make(nameSet)
	freeVariables := make(nameSet)
	for _, name := range scope.SymbolOrder {
		symbol := scope.Symbols[name]
		switch {
		case symbol.Flags&GlobalDeclaration != 0:
			symbol.Resolution = GlobalExplicit
			currentGlobals[name] = struct{}{}
		case symbol.Flags&NonlocalDeclaration != 0:
			if !hasName(enclosingBindings, name) {
				return nil, state.syntaxError(symbol.Declaration, "no binding for nonlocal %q found", name)
			}
			if hasName(typeParameters, name) {
				return nil, state.syntaxError(
					symbol.Declaration,
					"nonlocal binding not allowed for type parameter %q",
					name,
				)
			}
			symbol.Resolution = Free
			freeVariables[name] = struct{}{}
		case scope.redirectedBindings[name] == GlobalExplicit:
			symbol.Resolution = GlobalExplicit
			currentGlobals[name] = struct{}{}
		case scope.redirectedBindings[name] == Free:
			symbol.Resolution = Free
			freeVariables[name] = struct{}{}
		case isBinding(symbol.Flags):
			symbol.Resolution = Local
			localBindings[name] = struct{}{}
		case visibleClass != nil && visibleClass.Symbols[name] != nil &&
			(isBinding(visibleClass.Symbols[name].Flags) || visibleClass.Symbols[name].Flags&GlobalDeclaration != 0):
			if visibleClass.Symbols[name].Flags&GlobalDeclaration != 0 {
				symbol.Resolution = GlobalExplicit
			} else {
				symbol.Resolution = GlobalImplicit
			}
		case hasName(enclosingGlobals, name):
			symbol.Resolution = GlobalImplicit
		case hasName(enclosingBindings, name):
			symbol.Resolution = Free
			freeVariables[name] = struct{}{}
		default:
			symbol.Resolution = GlobalImplicit
		}
	}

	childBindings := cloneNames(enclosingBindings)
	switch scope.Kind {
	case ModuleScope:
	case ClassScope:
		childBindings["__class__"] = struct{}{}
		childBindings["__classdict__"] = struct{}{}
	default:
		addNames(childBindings, localBindings)
	}
	removeNames(childBindings, currentGlobals)
	childGlobals := cloneNames(enclosingGlobals)
	addNames(childGlobals, currentGlobals)
	removeNames(childGlobals, localBindings)
	childTypeParameters := cloneNames(typeParameters)
	for name := range localBindings {
		if scope.Symbols[name].Flags&TypeParameter != 0 {
			childTypeParameters[name] = struct{}{}
		} else {
			delete(childTypeParameters, name)
		}
	}
	removeNames(childTypeParameters, currentGlobals)

	for _, child := range scope.Children {
		childVisibleClass := (*Scope)(nil)
		if child.Flags&CanSeeClassScope != 0 {
			if scope.Kind == ClassScope {
				childVisibleClass = scope
			} else {
				childVisibleClass = visibleClass
			}
		}
		childFree, err := state.analyzeScope(
			child, childBindings, childGlobals, childTypeParameters, childVisibleClass,
		)
		if err != nil {
			return nil, err
		}
		for _, name := range sortedNames(childFree) {
			if scope.Kind == ClassScope {
				switch name {
				case "__class__":
					scope.Flags |= NeedsClassClosure
					continue
				case "__classdict__":
					scope.Flags |= NeedsClassDict
					continue
				}
				if _, local := localBindings[name]; local {
					if hasName(enclosingBindings, name) {
						scope.Symbols[name].Flags |= FreeThroughClass
						freeVariables[name] = struct{}{}
					}
					continue
				}
				if hasName(typeParameters, name) {
					freeVariables[name] = struct{}{}
					continue
				}
			}
			if _, local := localBindings[name]; local {
				scope.Symbols[name].Resolution = Cell
				continue
			}
			if hasName(currentGlobals, name) {
				continue
			}
			if hasName(enclosingBindings, name) {
				ensureFreeSymbol(scope, name)
				freeVariables[name] = struct{}{}
			}
		}
	}
	return freeVariables, nil
}

func sortedNames(names nameSet) []string {
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	return ordered
}

func ensureFreeSymbol(scope *Scope, name string) {
	if symbol := scope.Symbols[name]; symbol != nil {
		symbol.Resolution = Free
		return
	}
	scope.Symbols[name] = &Symbol{Name: name, Resolution: Free}
	scope.SymbolOrder = append(scope.SymbolOrder, name)
}

func cloneNames(names nameSet) nameSet {
	cloned := make(nameSet, len(names))
	addNames(cloned, names)
	return cloned
}

func addNames(destination, source nameSet) {
	for name := range source {
		destination[name] = struct{}{}
	}
}

func removeNames(destination, names nameSet) {
	for name := range names {
		delete(destination, name)
	}
}

func hasName(names nameSet, name string) bool {
	_, ok := names[name]
	return ok
}
