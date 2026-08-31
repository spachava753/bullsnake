# case: generic function parameters do not leak
# error: NameError
# message: "name 'T' is not defined"
def hidden[T]():
    return T
T
