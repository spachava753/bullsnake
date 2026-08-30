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
