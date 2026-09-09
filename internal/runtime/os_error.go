package runtime

var (
	osErrorType                = &exceptionTypeValue{name: "OSError", base: exceptionType}
	permissionErrorType        = &exceptionTypeValue{name: "PermissionError", base: osErrorType}
	fileNotFoundErrorType      = &exceptionTypeValue{name: "FileNotFoundError", base: osErrorType}
	fileExistsErrorType        = &exceptionTypeValue{name: "FileExistsError", base: osErrorType}
	notADirectoryErrorType     = &exceptionTypeValue{name: "NotADirectoryError", base: osErrorType}
	isADirectoryErrorType      = &exceptionTypeValue{name: "IsADirectoryError", base: osErrorType}
	blockingIOErrorType        = &exceptionTypeValue{name: "BlockingIOError", base: osErrorType}
	interruptedErrorType       = &exceptionTypeValue{name: "InterruptedError", base: osErrorType}
	timeoutErrorType           = &exceptionTypeValue{name: "TimeoutError", base: osErrorType}
	connectionErrorType        = &exceptionTypeValue{name: "ConnectionError", base: osErrorType}
	brokenPipeErrorType        = &exceptionTypeValue{name: "BrokenPipeError", base: connectionErrorType}
	connectionAbortedErrorType = &exceptionTypeValue{name: "ConnectionAbortedError", base: connectionErrorType}
	connectionRefusedErrorType = &exceptionTypeValue{name: "ConnectionRefusedError", base: connectionErrorType}
	connectionResetErrorType   = &exceptionTypeValue{name: "ConnectionResetError", base: connectionErrorType}
	childProcessErrorType      = &exceptionTypeValue{name: "ChildProcessError", base: osErrorType}
	unsupportedOperationType   = &exceptionTypeValue{name: "UnsupportedOperation", module: "io", base: osErrorType, additionalBase: valueErrorType}
)

// errnoExceptionType uses a fixed POSIX/Linux errno vocabulary, independent of
// the embedding process. Provider errors are translated into that vocabulary.
func errnoExceptionType(number int64) *exceptionTypeValue {
	switch number {
	case 1, 13:
		return permissionErrorType
	case 2:
		return fileNotFoundErrorType
	case 4:
		return interruptedErrorType
	case 10:
		return childProcessErrorType
	case 11, 114, 115:
		return blockingIOErrorType
	case 17:
		return fileExistsErrorType
	case 20:
		return notADirectoryErrorType
	case 21:
		return isADirectoryErrorType
	case 32, 108:
		return brokenPipeErrorType
	case 103:
		return connectionAbortedErrorType
	case 104:
		return connectionResetErrorType
	case 110:
		return timeoutErrorType
	case 111:
		return connectionRefusedErrorType
	default:
		return osErrorType
	}
}

// initializeOSError follows the two-to-five positional argument form, retaining
// filenames separately and selecting subclasses only for an exact OSError call.
func (exception *Exception) initializeOSError(arguments []Value) {
	if len(arguments) < 2 || len(arguments) > 5 {
		return
	}
	exception.fields = newNamespace()
	exception.fields.values["errno"] = arguments[0]
	exception.fields.values["strerror"] = arguments[1]
	if number, ok := arguments[0].(*intValue); ok && number.value.IsInt64() && exception.class == osErrorType && exception.userClass == nil {
		exception.class = errnoExceptionType(number.value.Int64())
	}
	if len(arguments) < 3 || arguments[2] == None {
		return
	}
	if exception.class == blockingIOErrorType {
		if _, ok := arguments[2].(*intValue); ok {
			exception.fields.values["characters_written"] = arguments[2]
			return
		}
	}
	exception.fields.values["filename"] = arguments[2]
	if len(arguments) == 5 {
		exception.fields.values["filename2"] = arguments[4]
	}
	exception.args = &tupleValue{elements: exception.args.elements[:2:2]}
}

// osErrorMessage formats structured errno and optional Python filenames,
// leaving ordinary argument-only exceptions to their base representation.
func (exception *Exception) osErrorMessage() (string, bool) {
	if exception.fields == nil || !exception.class.isSubclassOf(osErrorType) {
		return "", false
	}
	number, found := exception.fields.get("errno")
	if !found {
		return "", false
	}
	text := "[Errno " + valueText(number) + "] " + valueText(exception.fields.values["strerror"])
	if filename, found := exception.fields.get("filename"); found && filename != None {
		text += ": " + filename.Repr()
		if other, found := exception.fields.get("filename2"); found && other != None {
			text += " -> " + other.Repr()
		}
	}
	return text, true
}
