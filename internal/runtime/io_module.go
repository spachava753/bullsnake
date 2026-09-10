package runtime

// initializeIO exposes implemented in-memory I/O and the same exception classes
// used by host adapters. It does not acquire any host capability.
func initializeIO(_ *Runtime, module *Module) (*Exception, error) {
	initializeIOBases(module)
	initializeStringIOClass(module)
	module.globals.values["UnsupportedOperation"] = unsupportedOperationType
	module.globals.values["BlockingIOError"] = blockingIOErrorType
	return nil, nil
}
