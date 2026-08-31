# Runtime errors for pure builtins.
# case: len missing argument
# error: TypeError
# message: "len() takes exactly one argument (0 given)"
len()
# ---
# case: len extra argument
# error: TypeError
# message: "len() takes exactly one argument (2 given)"
len([], [])
# ---
# case: len keyword argument
# error: TypeError
# message: "len() takes no keyword arguments"
len(obj=[])
# ---
# case: value without length protocol
# error: TypeError
# message: "object of type 'NoneType' has no len()"
len(None)
# ---
# case: disabled length protocol
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledLength:
    __len__ = None

len(DisabledLength())
# ---
# case: non-index length result
# error: TypeError
# message: "'float' object cannot be interpreted as an integer"
class FloatLength:
    def __len__(self):
        return 1.5

len(FloatLength())
# ---
# case: negative length result
# error: ValueError
# message: "__len__() should return >= 0"
class NegativeLength:
    def __len__(self):
        return -1

len(NegativeLength())
# ---
# case: overflowing length result
# error: OverflowError
# message: "cannot fit 'int' into an index-sized integer"
class HugeLength:
    def __len__(self):
        return 1 << 100

len(HugeLength())
# ---
# case: length method exception
# error: ValueError
# message: "length failed"
class FailingLength:
    def __len__(self):
        raise ValueError('length failed')

len(FailingLength())
# ---
# case: iter missing argument
# error: TypeError
# message: "iter expected at least 1 argument, got 0"
iter()
# ---
# case: iter extra arguments
# error: TypeError
# message: "iter expected at most 2 arguments, got 3"
iter([], None, None)
# ---
# case: iter keyword argument
# error: TypeError
# message: "iter() takes no keyword arguments"
iter(iterable=[])
# ---
# case: non-iterable value
# error: TypeError
# message: "'int' object is not iterable"
iter(1)
# ---
# case: disabled iteration protocol
# error: TypeError
# message: "'DisabledIteration' object is not iterable"
class DisabledIteration:
    __iter__ = None

iter(DisabledIteration())
# ---
# case: invalid iterator result
# error: TypeError
# message: "iter() returned non-iterator of type 'int'"
class InvalidIteration:
    def __iter__(self):
        return 1

iter(InvalidIteration())
# ---
# case: iteration method exception
# error: ValueError
# message: "iteration failed"
class FailingIteration:
    def __iter__(self):
        raise ValueError('iteration failed')

iter(FailingIteration())
# ---
# case: callable sentinel iterator boundary
# error: NotImplementedError
# message: "iter() callable-sentinel form is not supported"
def produce():
    return None

iter(produce, None)
