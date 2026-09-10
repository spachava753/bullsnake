package runtime

// initializeIO exposes implemented in-memory I/O and the same exception classes
// used by host adapters. It does not acquire any host capability.
func initializeIO(_ *Runtime, module *Module) (*Exception, error) {
	initializeIOBases(module)
	initializeRawIOClass(module)
	initializeBufferedIOClass(module)
	initializeBytesIOClass(module)
	initializeStringIOClass(module)
	initializeNewlineDecoder(module)
	initializeBufferedType(module, "BufferedReader", []string{"read", "read1", "readinto", "readinto1", "readline", "peek"}, executeBufferedReader)
	initializeBufferedType(module, "BufferedWriter", []string{"write", "truncate"}, executeBufferedWriter)
	initializeBufferedType(module, "BufferedRandom", []string{"read", "read1", "readinto", "readinto1", "readline", "peek", "write", "truncate"}, executeBufferedRandom)
	initializeBufferedPair(module)
	initializeTextWrapper(module)
	initializeIOPermissions(module)
	module.globals.values["UnsupportedOperation"] = unsupportedOperationType
	module.globals.values["BlockingIOError"] = blockingIOErrorType
	return nil, nil
}
