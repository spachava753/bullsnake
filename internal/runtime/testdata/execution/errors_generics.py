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
