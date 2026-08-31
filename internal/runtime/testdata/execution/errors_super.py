# Super construction and lookup errors.
# case: zero argument super outside a method
# error: RuntimeError
# message: "super(): no arguments"
super()
# ---
# case: zero argument super after deleting first argument
# error: RuntimeError
# message: "super(): arg[0] deleted"
class DeletedSuperArgument:
    def method(self):
        del self
        return super()

DeletedSuperArgument().method()
# ---
# case: explicit super requires a type
# error: TypeError
# message: "super() argument 1 must be a type, not int"
super(1, None)
# ---
# case: explicit super receiver must match
# error: TypeError
# message: "super(type, obj): obj (instance of OtherSuperClass) is not an instance or subtype of type (ExpectedSuperClass)."
class ExpectedSuperClass:
    pass

class OtherSuperClass:
    pass

super(ExpectedSuperClass, OtherSuperClass())
# ---
# case: missing super attribute
# error: AttributeError
# message: "'super' object has no attribute 'missing'"
class SuperMissingBase:
    pass

class SuperMissingChild(SuperMissingBase):
    pass

super(SuperMissingChild, SuperMissingChild()).missing
