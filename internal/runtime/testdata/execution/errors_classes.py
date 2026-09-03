# Runtime exception cases for classes.
# case: non-type class base
# error: TypeError
# message: "class base is not a type"
class Broken(1):
    pass
# ---
# case: initializer return value
# error: TypeError
# message: "__init__() should return None, not 'int'"
class Broken:
    def __init__(self):
        return 1
answer = Broken()
# ---
# case: delete missing instance attribute
# error: AttributeError
# message: "'Empty' object has no attribute 'missing'"
class Empty:
    pass
value = Empty()
del value.missing
# ---
# case: delete missing type attribute
# error: AttributeError
# message: "type object 'Empty' has no attribute 'missing'"
class Empty:
    pass
del Empty.missing
# ---
# case: constructor arguments without init
# error: TypeError
# message: "Empty() takes no arguments"
class Empty:
    pass
answer = Empty(1)
# ---
# case: missing instance attribute
# error: AttributeError
# message: "'Empty' object has no attribute 'missing'"
class Empty:
    pass
answer = Empty().missing
# ---
# case: missing type attribute
# error: AttributeError
# message: "type object 'Empty' has no attribute 'missing'"
class Empty:
    pass
answer = Empty.missing
