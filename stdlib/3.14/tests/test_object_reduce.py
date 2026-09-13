"""Exercise real reduction tuples and unchanged copyreg reconstruction."""
class Sample:
    def __init__(self):
        self.number = 42
value = Sample()
# Cold reduction must import copyreg before obtaining default state.
r = value.__reduce_ex__(4)
import copyreg
assert r == (copyreg.__newobj__, (Sample,), {'number': 42}, None, None)
restored = r[0](*r[1])
assert type(restored) is Sample
assert not hasattr(restored, 'number')
for key, item in r[2].items():
    setattr(restored, key, item)
assert restored.number == 42
assert r[2] is value.__getstate__()
for protocol in (-1, 0, 1, 2, 5):
    reduced = value.__reduce_ex__(protocol)
    if protocol < 2:
        assert reduced == (copyreg._reconstructor, (Sample, object, None), r[2])
    else:
        assert reduced == r
assert value.__reduce__() == value.__reduce_ex__(0)
assert object().__reduce_ex__(4) == (copyreg.__newobj__, (object,), None, None, None)

class Override:
    def __reduce__(self):
        return 'override'
a = Override()
assert a.__reduce_ex__(4) == 'override'
a.__reduce__ = lambda: 'instance with class override'
assert a.__reduce_ex__(4) == 'instance with class override'
value.__reduce__ = lambda: 'ignored instance-only override'
assert value.__reduce_ex__(4)[0] is copyreg.__newobj__
del value.__reduce__
class Delegating:
    def __reduce__(self):
        return object.__reduce__(self)
assert Delegating().__reduce_ex__(4) == (copyreg._reconstructor, (Delegating, object, None))

calls = []
class Arguments:
    def __new__(cls, number, *, label='default'):
        calls.append(('new', number, label))
        result = object.__new__(cls)
        result.number = number
        result.label = label
        return result
    def __getnewargs_ex__(self):
        calls.append('arguments')
        return (self.number,), {'label': self.label}
    def __getnewargs__(self):
        assert False
    def __getstate__(self):
        calls.append('state')
        return 'custom state'
a = Arguments.__new__(Arguments, 7, label='named')
calls = []
a.__getnewargs_ex__ = lambda: ((), {})  # special lookup ignores instance attributes
r = a.__reduce_ex__(2)
assert calls == ['arguments', 'state']
assert r == (copyreg.__newobj_ex__, (Arguments, (7,), {'label': 'named'}), 'custom state', None, None)
b = r[0](*r[1])
assert b.number == 7 and b.label == 'named'
assert calls[-1] == ('new', 7, 'named')
class Positional(Arguments):
    def __getnewargs_ex__(self):
        return (8,), {}
b = Positional.__new__(Positional, 8)
r = b.__reduce_ex__(4)
assert r[:2] == (copyreg.__newobj__, (Positional, 8))
assert r[0](*r[1]).number == 8
class OldArguments:
    def __getnewargs__(self):
        return (1, 2)
assert OldArguments().__reduce_ex__(4)[1] == (OldArguments, 1, 2)

class Mapping(dict):
    pass
mapping = Mapping({'one': 1})
mapping.label = 'extra'
r = mapping.__reduce_ex__(4)
assert r[:4] == (copyreg.__newobj__, (Mapping,), {'label': 'extra'}, None)
assert iter(r[4]) is r[4]
mapping['two'] = 2
try:
    next(r[4])
except RuntimeError:
    pass
else:
    assert False
r = mapping.__reduce_ex__(4)
assert list(r[4]) == [('one', 1), ('two', 2)]
assert type(r[0](*r[1])) is Mapping
for protocol in (0, 1):
    old = mapping.__reduce_ex__(protocol)
    assert old == (copyreg._reconstructor, (Mapping, dict, {'one': 1, 'two': 2}), {'label': 'extra'})
    clone = old[0](*old[1])
    assert type(clone) is Mapping
    assert clone == mapping
class CustomItems(dict):
    def items(self):
        return iter((('override', 99),))
assert list(CustomItems().__reduce_ex__(4)[4]) == [('override', 99)]

class Protocol:
    def __index__(self):
        calls.append('index')
        return 4
calls = []
assert value.__reduce_ex__(Protocol())[0] is copyreg.__newobj__
assert calls == ['index']
for protocol in ('4', 4.0, 1 << 40):
    try:
        value.__reduce_ex__(protocol)
    except (TypeError, OverflowError):
        pass
    else:
        assert False
