# case: reversed calls class-level methods without forcing the result to iterate
calls = []
class Base:
    def __reversed__(self):
        calls.append('reverse')
        return iter([3, 2, 1])
class Child(Base):
    pass
value = Child()
value.__reversed__ = lambda: [9]
assert list(reversed(value)) == [3, 2, 1]
assert calls == ['reverse']
class ReturnsValue:
    def __reversed__(self):
        return 42
assert reversed(ReturnsValue()) == 42
class Disabled(Base):
    __reversed__ = None
try:
    reversed(Disabled())
    assert False
except TypeError:
    pass
class Broken:
    def __reversed__(self):
        raise ValueError('reverse failed')
try:
    reversed(Broken())
    assert False
except ValueError as error:
    assert str(error) == 'reverse failed'
