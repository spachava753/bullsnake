# case: type missing arguments
# error: TypeError
# message: "type() takes 1 or 3 arguments"
type()
# ---
# case: type invalid argument count
# error: TypeError
# message: "type() takes 1 or 3 arguments"
type(None, None)
# ---
# case: type keyword argument
# error: TypeError
# message: "type() takes 1 or 3 arguments"
type(object=None)
# ---
# case: dynamic type construction boundary
# error: NotImplementedError
# message: "three-argument type() is not supported"
type('Dynamic', (), {})
