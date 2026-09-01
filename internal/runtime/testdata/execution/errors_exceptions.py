# Runtime exception cases for exceptions.
# case: assertion without message
# error: AssertionError
# message: ""
assert False
# ---
# case: assertion with message
# error: AssertionError
# message: "broken"
assert False, 'broken'
# ---
# case: invalid explicit raise
# error: TypeError
# message: "exceptions must derive from BaseException"
raise None
# ---
# case: unmatched typed handler
# error: AssertionError
# message: "unmatched"
try:
    assert False, 'unmatched'
except TypeError:
    pass
# ---
# case: invalid exception handler type
# error: TypeError
# message: "catching classes that do not inherit from BaseException is not allowed"
try:
    missing_for_invalid_handler
except 1:
    pass
# ---
# case: invalid exception handler tuple member
# error: TypeError
# message: "catching classes that do not inherit from BaseException is not allowed"
try:
    missing_for_invalid_tuple
except (NameError, 1):
    pass
# ---
# case: bare raise without active exception
# error: RuntimeError
# message: "No active exception to reraise"
raise
# ---
# case: handler continue clears active exception
# error: RuntimeError
# message: "No active exception to reraise"
for item in (1,):
    try:
        missing_before_continue
    except NameError:
        continue
raise
# ---
# case: invalid explicit exception cause
# error: TypeError
# message: "exception causes must derive from BaseException"
raise ValueError('outer') from 42
# ---
# case: custom exception initializer is unsupported
# error: TypeError
# message: "custom exception initializers are not supported"
class CustomError(Exception):
    def __init__(self):
        pass
CustomError()
# ---
# case: exception group requires two arguments
# error: TypeError
# message: "BaseExceptionGroup.__new__() takes exactly 2 arguments (1 given)"
ExceptionGroup('missing children')
# ---
# case: exception group message must be text
# error: TypeError
# message: "BaseExceptionGroup.__new__() argument 1 must be str, not int"
ExceptionGroup(1, [ValueError('bad')])
# ---
# case: exception group children must be a sequence
# error: TypeError
# message: "second argument (exceptions) must be a sequence"
ExceptionGroup('not a sequence', None)
# ---
# case: exception group children cannot be empty
# error: ValueError
# message: "second argument (exceptions) must be a non-empty sequence"
ExceptionGroup('empty', [])
# ---
# case: exception group children must be exceptions
# error: ValueError
# message: "Item 0 of second argument (exceptions) is not an exception"
ExceptionGroup('invalid member', [ValueError])
# ---
# case: exception groups reject base-only exceptions
# error: TypeError
# message: "Cannot nest BaseExceptions in an ExceptionGroup"
ExceptionGroup('base child', [BaseException('stop')])
# ---
# case: exception group class form requires constructor arguments
# error: TypeError
# message: "BaseExceptionGroup.__new__() takes exactly 2 arguments (0 given)"
raise ExceptionGroup
# ---
# case: uncaught exception groups report their child count
# error: ExceptionGroup
# message: "batch (2 sub-exceptions)"
raise ExceptionGroup('batch', [ValueError('bad'), TypeError('wrong')])
# ---
# case: except star rejects non-exception handler values
# error: TypeError
# message: "catching classes that do not inherit from BaseException is not allowed"
try:
    raise ValueError('bad handler')
except* 42:
    pass
# ---
# case: except star rejects exception group handler classes
# error: TypeError
# message: "catching ExceptionGroup with except* is not allowed. Use except instead."
try:
    raise ExceptionGroup('group', [ValueError('leaf')])
except* ExceptionGroup:
    pass
# ---
# case: except star clears a bound handler name
# error: NameError
# message: "name 'caught' is not defined"
try:
    raise ExceptionGroup('group', [ValueError('leaf')])
except* ValueError as caught:
    saved = caught
caught
# ---
# case: exception with traceback missing argument
# error: TypeError
# message: "BaseException.with_traceback() takes exactly one argument (0 given)"
ValueError('missing').with_traceback()
# ---
# case: exception with traceback extra argument
# error: TypeError
# message: "BaseException.with_traceback() takes exactly one argument (2 given)"
ValueError('extra').with_traceback(None, None)
# ---
# case: exception with traceback keyword argument
# error: TypeError
# message: "ValueError.with_traceback() takes no keyword arguments"
ValueError('keyword').with_traceback(tb=None)
# ---
# case: exception with traceback invalid value
# error: TypeError
# message: "__traceback__ must be a traceback or None"
ValueError('invalid').with_traceback(1)
