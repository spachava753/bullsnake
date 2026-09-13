package runtime

// classHasCustomKeySlots guards identity-only native key operations until they
// can suspend for metaclass equality and hashing, including inherited overrides.
func classHasCustomKeySlots(class *typeValue) bool {
	if class.metaclass == nil {
		return false
	}
	for _, name := range []string{"__eq__", "__hash__"} {
		if _, found := class.metaclass.lookup(name); found {
			return true
		}
	}
	return false
}
