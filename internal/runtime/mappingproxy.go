package runtime

// mappingProxyValue exposes a dictionary without granting mutation through the
// proxy. Class mutations update this same storage, including active views.
type mappingProxyValue struct {
	dictionary *dictValue
}

func (*mappingProxyValue) TypeName() string   { return "mappingproxy" }
func (proxy *mappingProxyValue) Repr() string { return "mappingproxy(" + proxy.dictionary.Repr() + ")" }
func (*mappingProxyValue) isValue()           {}

func (class *typeValue) namespaceProxy() *mappingProxyValue {
	if class.namespaceDictionary == nil {
		class.namespaceDictionary = &dictValue{}
		for _, name := range class.namespaceOrder {
			if value, found := class.namespace.get(name); found {
				class.namespaceDictionary.set(&stringValue{value: name}, value)
			}
		}
	}
	return &mappingProxyValue{dictionary: class.namespaceDictionary}
}

func executeMappingProxyAttributeLoad(caller *frame, instruction int, proxy *mappingProxyValue, name string) (instructionOutcome, error) {
	switch name {
	case "get", "keys", "values", "items", "copy":
		return executeDictionaryAttributeLoad(caller, instruction, proxy.dictionary, name)
	default:
		return raiseOutcome(newException("AttributeError", "'mappingproxy' object has no attribute '"+name+"'")), nil
	}
}

func (class *typeValue) deleteAttribute(name string) {
	delete(class.namespace.values, name)
	for index, key := range class.namespaceOrder {
		if key == name {
			class.namespaceOrder = append(class.namespaceOrder[:index], class.namespaceOrder[index+1:]...)
			break
		}
	}
	if class.namespaceDictionary != nil {
		class.namespaceDictionary.delete(&stringValue{value: name})
	}
}
