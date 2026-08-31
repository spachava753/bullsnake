package resolver

import "strings"

// Mangle returns the runtime spelling of a source identifier in this scope.
func (scope *Scope) Mangle(name string) string {
	return manglePrivate(scope.PrivateName, name)
}

// manglePrivate applies Python's class-private name rewrite.
func manglePrivate(className, name string) string {
	if className == "" || !strings.HasPrefix(name, "__") || strings.HasSuffix(name, "__") || strings.Contains(name, ".") {
		return name
	}
	className = strings.TrimLeft(className, "_")
	if className == "" {
		return name
	}
	return "_" + className + name
}
