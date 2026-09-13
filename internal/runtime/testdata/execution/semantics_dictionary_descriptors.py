# case: native dictionary descriptors operate on real ordered storage
mapping = dict.__new__(dict, ignored=1)
assert mapping == {}
assert dict.__init__(mapping, {'a': 1}, b=2) is None
assert dict.__init__(mapping) is None
assert mapping == {'a': 1, 'b': 2}
assert dict.__setitem__(mapping, 'c', 3) is None
assert dict.__getitem__(mapping, 'c') == 3
assert dict.get(mapping, 'missing', 4) == 4
assert dict.setdefault(mapping, 'a', 10) == 1
shared = []
assert mapping.setdefault('new', shared) is shared
assert mapping.setdefault('new') is shared
assert mapping.setdefault('none') is None
assert dict.__delitem__(mapping, 'b') is None
assert list(dict.keys(mapping)) == ['a', 'c', 'new', 'none']
assert dict.copy(mapping) == mapping
assert dict.copy(mapping) is not mapping
assert dict.__repr__({'a': 1}) == "{'a': 1}"
assert dict.__eq__({'a': 1}, {'a': 1})
assert dict.__ne__({'a': 1}, {'b': 1})
assert dict.__eq__({}, 1) is NotImplemented
assert dict.pop(mapping, 'c') == 3
assert dict.clear(mapping) is None
assert mapping == {}
assert dict.__setitem__.__get__(mapping, dict)('x', 9) is None
assert mapping == {'x': 9}

# ---
# case: explicit dict equality descriptors resume Python value callbacks
calls = []
class Value:
    def __eq__(self, other):
        calls.append(other)
        return other == 3
value = Value()
assert dict.__eq__({'a': value}, {'a': 3})
assert not dict.__ne__({'a': value}, {'a': 3})
assert calls == [3, 3]

# ---
# case: dictionary slot errors preserve the receiver
mapping = {'a': 1}
try:
    dict.__getitem__(mapping, 'missing')
    assert False
except KeyError as error:
    assert str(error) == "'missing'"
try:
    dict.__delitem__(mapping, 'missing')
    assert False
except KeyError as error:
    assert str(error) == "'missing'"
try:
    dict.__setitem__([], 'a', 2)
    assert False
except TypeError as error:
    assert str(error) == "descriptor '__setitem__' requires a 'dict' object"
assert mapping == {'a': 1}
