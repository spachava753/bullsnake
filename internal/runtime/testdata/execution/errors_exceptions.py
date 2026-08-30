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
