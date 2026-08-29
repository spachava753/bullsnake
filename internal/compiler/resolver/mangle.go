package resolver

import "strings"

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
