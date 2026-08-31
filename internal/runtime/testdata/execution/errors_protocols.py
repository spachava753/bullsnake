# Runtime exception cases for user-defined object protocols.
# case: non-boolean bool result
# error: TypeError
# message: "__bool__ should return bool, returned int"
class InvalidBool:
    def __bool__(self):
        return 1

if InvalidBool():
    pass
# ---
# case: disabled bool method
# error: TypeError
# message: "'DisabledBool' cannot be interpreted as a boolean"
class DisabledBool:
    __bool__ = None

if DisabledBool():
    pass
# ---
# case: non-integer length result
# error: TypeError
# message: "'float' object cannot be interpreted as an integer"
class FloatLength:
    def __len__(self):
        return 1.5

if FloatLength():
    pass
# ---
# case: negative length result
# error: ValueError
# message: "__len__() should return >= 0"
class NegativeLength:
    def __len__(self):
        return -1

if NegativeLength():
    pass
# ---
# case: oversized length result
# error: OverflowError
# message: "cannot fit 'int' into an index-sized integer"
class HugeLength:
    def __len__(self):
        return 100000000000000000000000000000000000000000000000000000000

if HugeLength():
    pass
