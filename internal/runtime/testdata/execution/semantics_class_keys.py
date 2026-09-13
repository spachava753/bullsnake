# case: classes are identity keys in dictionaries sets and frozen sets
class First:
    pass
class Second:
    pass
mapping = {First: 'first', Second: 'second', int: 'int', ValueError: 'error'}
assert len(mapping) == 4
assert mapping[First] == 'first'
assert mapping[int] == 'int'
assert mapping[ValueError] == 'error'
mapping[First] = 'updated'
assert mapping[First] == 'updated'
assert set(mapping) == {First, Second, int, ValueError}
assert frozenset({First, int}) == frozenset({int, First})
assert hash(frozenset({First, int})) == hash(frozenset({int, First}))
assert {(First, int): 42}[(First, int)] == 42
assert First in {First}
assert Second not in {First}
assert {First, int} & {First, str} == {First}
assert {frozenset({First, int}): 3}[frozenset({int, First})] == 3

# ---
# case: classes with matching names remain distinct identity keys
def factory():
    class Same:
        pass
    return Same
first = factory()
second = factory()
assert first is not second
mapping = {first: 1, second: 2}
assert len(mapping) == 2
assert mapping[first] == 1
assert mapping[second] == 2
before = hash(first)
first.__module__ = 'changed'
assert hash(first) == before
assert mapping[first] == 1

# ---
# case: unsupported metaclass key overrides are rejected rather than ignored
calls = []
class Meta(type):
    def __hash__(cls):
        calls.append('hash')
        return 1
class Custom(metaclass=Meta):
    pass
try:
    {Custom}
    assert False
except TypeError:
    pass
try:
    {(Custom,): 1}
    assert False
except TypeError:
    pass
assert calls == []

# ---
# case: plain Python and native functions are identity keys
def factory():
    def function():
        pass
    return function
first = factory()
second = factory()
assert len({first, second, abs}) == 3
mapping = {first: 1, second: 2, abs: 3}
assert mapping[first] == 1
assert mapping[second] == 2
assert mapping[abs] == 3
first.__doc__ = 'changed'
assert mapping[first] == 1
