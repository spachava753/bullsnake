# case: generic function parameters do not leak
# error: NameError
# message: "name 'T' is not defined"
def hidden[T]():
    return T
T

# ---
# case: generic function annotation failures remain lazy
# error: NameError
# message: "name 'Missing' is not defined"
def missing_annotation[T](value: Missing):
    return value

missing_annotation.__annotations__

# ---
# case: generic function defaults cannot see their type parameters
# error: NameError
# message: "name 'T' is not defined"
def invalid_default[T](value=T):
    return value

# ---
# case: generic function decorators cannot see their type parameters
# error: NameError
# message: "name 'T' is not defined"
@T
def invalid_decorator[T]():
    return None

# ---
# case: variadic generic functions still require ordinary parameters
# error: TypeError
# message: "require() missing 1 required positional argument: 'head'"
def require[T](head, *items, **options):
    return head

require()

# ---
# case: generic function TypeVar bound failures remain lazy
# error: NameError
# message: "name 'MissingBound' is not defined"
def missing_bound[T: MissingBound]():
    return T

missing_bound.__type_params__[0].__bound__
