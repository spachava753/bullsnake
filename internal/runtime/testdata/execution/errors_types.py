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
# ---
# case: isinstance missing argument
# error: TypeError
# message: "isinstance expected 2 arguments, got 1"
isinstance(None)
# ---
# case: isinstance extra argument
# error: TypeError
# message: "isinstance expected 2 arguments, got 3"
isinstance(None, type(None), type(None))
# ---
# case: isinstance keyword arguments
# error: TypeError
# message: "isinstance() takes no keyword arguments"
isinstance(obj=None, class_or_tuple=type(None))
# ---
# case: isinstance invalid class
# error: TypeError
# message: "isinstance() arg 2 must be a type, a tuple of types, or a union"
isinstance(None, 1)
# ---
# case: isinstance invalid nested class
# error: TypeError
# message: "isinstance() arg 2 must be a type, a tuple of types, or a union"
isinstance(None, (str, (int, 1)))
# ---
# case: issubclass missing argument
# error: TypeError
# message: "issubclass expected 2 arguments, got 1"
issubclass(type(None))
# ---
# case: issubclass extra argument
# error: TypeError
# message: "issubclass expected 2 arguments, got 3"
issubclass(type(None), type(None), type(None))
# ---
# case: issubclass keyword arguments
# error: TypeError
# message: "issubclass() takes no keyword arguments"
issubclass(cls=type(None), class_or_tuple=type(None))
# ---
# case: issubclass invalid first class
# error: TypeError
# message: "issubclass() arg 1 must be a class"
issubclass(None, type(None))
# ---
# case: issubclass invalid second class
# error: TypeError
# message: "issubclass() arg 2 must be a class, a tuple of classes, or a union"
issubclass(type(None), None)
# ---
# case: issubclass invalid nested class
# error: TypeError
# message: "issubclass() arg 2 must be a class, a tuple of classes, or a union"
issubclass(type(None), (str, (int, None)))
