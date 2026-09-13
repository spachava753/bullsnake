# case: dictionary subclasses retain native storage and Python attributes
class Mapping(dict):
    pass
mapping = Mapping({'a': 1}, b=2)
assert isinstance(mapping, dict)
assert issubclass(Mapping, dict)
assert type(mapping) is Mapping
assert Mapping.__bases__ == (dict,)
assert Mapping.__mro__ == (Mapping, dict, object)
assert mapping == {'a': 1, 'b': 2}
assert {'a': 1, 'b': 2} == mapping
assert len(mapping) == 2
assert list(mapping) == ['a', 'b']
assert 'a' in mapping
assert bool(mapping)
mapping.label = 'separate'
mapping['label'] = 3
assert mapping.label == 'separate'
assert mapping['label'] == 3
assert mapping.get('a') == 1
assert mapping.setdefault('a', 9) == 1
assert type(mapping.copy()) is dict
assert repr(mapping) == "{'a': 1, 'b': 2, 'label': 3}"
assert dict.__len__(mapping) == 3
assert dict.__getitem__(mapping, 'a') == 1
keys = mapping.keys()
mapping['c'] = 4
assert list(keys) == ['a', 'b', 'label', 'c']
del mapping['a']
assert mapping.pop('b') == 2
mapping.clear()
assert not mapping
try:
    hash(mapping)
    assert False
except TypeError as error:
    assert str(error) == "unhashable type: 'Mapping'"

# ---
# case: dict subclasses use Python overrides and explicit native slots bypass them
calls = []
class Mapping(dict):
    def __init__(self, source):
        calls.append('init')
        super().__init__(source)
    def __setitem__(self, key, value):
        calls.append((key, value))
        super().__setitem__(key, value + 1)
    def __getitem__(self, key):
        return super().__getitem__(key) * 2
    def __len__(self):
        return 10
mapping = Mapping({'a': 1})
assert calls == ['init']
mapping['b'] = 2
assert calls == ['init', ('b', 2)]
assert mapping['a'] == 2
assert dict.__getitem__(mapping, 'b') == 3
assert len(mapping) == 10
assert dict.__len__(mapping) == 2
mapping.update({'c': 5})
assert calls == ['init', ('b', 2)]
assert mapping.get('c') == 5

# ---
# case: dictionary subtype new and init share compatible-result rules
calls = []
class Mapping(dict):
    def __new__(cls, value):
        calls.append('new')
        return dict.__new__(cls)
    def __init__(self, value):
        calls.append('init')
        super().__init__(value)
mapping = Mapping({'a': 1})
assert calls == ['new', 'init']
assert mapping == {'a': 1}
class Foreign(dict):
    def __new__(cls):
        return 42
    def __init__(self):
        assert False
assert Foreign() == 42
class Child(Mapping):
    pass
child = Child({'b': 2})
assert child == {'b': 2}
assert Child.__mro__ == (Child, Mapping, dict, object)

# ---
# case: only dictionary subscription invokes a subclass missing hook
calls = []
class Mapping(dict):
    def __missing__(self, key):
        calls.append(key)
        return ('missing', key)
mapping = Mapping()
assert mapping['x'] == ('missing', 'x')
assert dict.__getitem__(mapping, 'y') == ('missing', 'y')
assert mapping.get('z') is None
assert 'z' not in mapping
assert calls == ['x', 'y']
