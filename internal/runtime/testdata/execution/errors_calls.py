# Runtime exception cases for calls.
# case: missing keyword-only arguments
# error: TypeError
# message: "configure() missing 2 required keyword-only arguments: 'required' and 'verbose'"
def configure(*, required, mode='safe', verbose):
    return required, mode, verbose
answer = configure()
# ---
# case: positional and keyword duplicate
# error: TypeError
# message: "combine() got multiple values for argument 'first'"
def combine(first, second):
    return first + second
answer = combine(1, first=2, second=3)
# ---
# case: positional-only keyword
# error: TypeError
# message: "combine() got some positional-only arguments passed as keyword arguments: 'first'"
def combine(first, /, second):
    return first + second
answer = combine(first=1, second=2)
# ---
# case: unexpected keyword
# error: TypeError
# message: "identity() got an unexpected keyword argument 'missing'"
def identity(value):
    return value
answer = identity(missing=1)
# ---
# case: duplicate expanded keyword
# error: TypeError
# message: "identity() got multiple values for keyword argument 'value'"
def identity(value):
    return value
answer = identity(value=1, **{'value': 2})
# ---
# case: non-mapping keyword expansion
# error: TypeError
# message: "identity() argument after ** must be a mapping, not int"
def identity(value):
    return value
answer = identity(**1)
# ---
# case: non-string keyword
# error: TypeError
# message: "identity() keywords must be strings"
def identity(value):
    return value
answer = identity(**{1: 2})
# ---
# case: missing argument after keywords
# error: TypeError
# message: "combine() missing 1 required positional argument: 'first'"
def combine(first, second):
    return first + second
answer = combine(second=2)
# ---
# case: missing positional argument after unpacking
# error: TypeError
# message: "add() missing 1 required positional argument: 'right'"
def add(left, right):
    return left + right
answer = add(*(1,))
# ---
# case: missing required argument before varargs
# error: TypeError
# message: "collect() missing 1 required positional argument: 'first'"
def collect(first, *items):
    return first, items
answer = collect()
# ---
# case: missing required argument before defaults
# error: TypeError
# message: "choose() missing 1 required positional argument: 'required'"
def choose(required, optional=2):
    return required + optional
answer = choose()
# ---
# case: too many arguments with defaults
# error: TypeError
# message: "choose() takes from 1 to 2 positional arguments but 3 were given"
def choose(required, optional=2):
    return required + optional
answer = choose(1, 2, 3)
# ---
# case: non-callable value
# error: TypeError
# message: "'int' object is not callable"
answer = 1()
# ---
# case: missing positional argument
# error: TypeError
# message: "add() missing 1 required positional argument: 'right'"
def add(left, right):
    return left + right
answer = add(1)
# ---
# case: too many positional arguments
# error: TypeError
# message: "add() takes 2 positional arguments but 3 were given"
def add(left, right):
    return left + right
answer = add(1, 2, 3)
