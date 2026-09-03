package runtime

type coroutineValue struct {
	frame              *frame
	name               string
	running            bool
	done               bool
	immediate          Value
	immediateException *Exception
	warned             bool
}

func (*coroutineValue) TypeName() string { return "coroutine" }
func (coroutine *coroutineValue) Repr() string {
	return "<coroutine object " + coroutine.name + ">"
}
func (*coroutineValue) isValue() {}

func (coroutine *coroutineValue) attribute(name string) (Value, bool) {
	if name != "close" {
		return nil, false
	}
	return nativeFunctionNamed("coroutine.close", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			if coroutine.running {
				return nil, newException("ValueError", "coroutine already executing"), nil
			}
			coroutine.clear()
			return None, nil, nil
		}), true
}

func (coroutine *coroutineValue) clear() {
	if coroutine.frame != nil {
		for index := range coroutine.frame.stack {
			coroutine.frame.stack[index] = nil
		}
		coroutine.frame.stack = nil
		coroutine.frame.fastLocals = nil
		coroutine.frame.deref = nil
		coroutine.frame.previous = nil
	}
	coroutine.done = true
}

type asyncGeneratorValue struct {
	frame *frame
	name  string
	done  bool
}

func (*asyncGeneratorValue) TypeName() string { return "async_generator" }
func (generator *asyncGeneratorValue) Repr() string {
	return "<async_generator object " + generator.name + ">"
}
func (*asyncGeneratorValue) isValue() {}

var _ attributeValue = (*coroutineValue)(nil)
