package runtime

// initializeTime exposes only the explicitly supplied performance counter.
func initializeTime(runtime *Runtime, module *Module) (*Exception, error) {
	module.globals.values["perf_counter"] = &builtinFunctionValue{
		name: "perf_counter",
		call: func(arguments []Value, keywords *dictValue) (Value, *Exception) {
			if exception := checkNativeArguments("perf_counter", arguments, keywords, 0, 0); exception != nil {
				return nil, exception
			}
			if runtime.counter == nil {
				return nil, newException("PermissionError", "performance counter is not configured")
			}
			return &floatValue{value: runtime.counter.PerfCounter().Seconds()}, nil
		},
	}
	return nil, nil
}
