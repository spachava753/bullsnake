# case: defining equality disables inherited hashing unless explicitly retained
class Base:
    def __hash__(self):
        return 17
class Equal(Base):
    def __eq__(self, other):
        return True
assert Equal.__dict__['__hash__'] is None
try:
    hash(Equal())
    assert False
except TypeError:
    pass
class Retained(Base):
    def __eq__(self, other):
        return True
    __hash__ = Base.__hash__
assert hash(Retained()) == 17
class Inherited(Retained):
    pass
assert '__hash__' not in Inherited.__dict__
assert hash(Inherited()) == 17
Dynamic = type('Dynamic', (Base,), {'__eq__': lambda self, other: True})
assert Dynamic.__dict__['__hash__'] is None
class Assigned(Base):
    pass
Assigned.__eq__ = lambda self, other: True
assert hash(Assigned()) == 17
del Equal.__eq__
assert Equal.__hash__ is None

# ---
# case: user hash results preserve signed hash-width integers
class Hashed:
    def __init__(self, value):
        self.value = value
    def __hash__(self):
        return self.value
for value in [(1 << 62) + 1, -(1 << 62) - 1, (1 << 63) - 1, -(1 << 63), 0]:
    assert hash(Hashed(value)) == value
assert hash(Hashed(-1)) == -2
assert hash(Hashed(1 << 80)) == hash(1 << 80)

# ---
# case: sys maxsize reflects supported native index width
import sys
assert sys.maxsize == (1 << 63) - 1
assert len(range(sys.maxsize)) == sys.maxsize
try:
    len(range(sys.maxsize + 1))
    assert False
except OverflowError:
    pass
