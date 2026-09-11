# Project-owned behavior tests against unchanged CPython 3.14.7 _collections_abc.
from _collections_abc import Mapping, MutableMapping, KeysView, ItemsView, ValuesView

class Data(MutableMapping):
    def __init__(self, source=()):
        self.data = dict(source)
    def __getitem__(self, key):
        return self.data[key]
    def __setitem__(self, key, value):
        self.data[key] = value
    def __delitem__(self, key):
        del self.data[key]
    def __iter__(self):
        return iter(self.data)
    def __len__(self):
        return len(self.data)

mapping = Data([('a', 1), ('b', 2)])
assert mapping.get('a') == 1
assert mapping.get('missing') is None
assert mapping.get('missing', 3) == 3
assert 'a' in mapping and 'c' not in mapping
keys, items, values = mapping.keys(), mapping.items(), mapping.values()
assert isinstance(keys, KeysView)
assert isinstance(items, ItemsView)
assert isinstance(values, ValuesView)
assert len(keys) == len(items) == len(values) == 2
assert list(keys) == ['a', 'b']
assert list(items) == [('a', 1), ('b', 2)]
assert list(values) == [1, 2]
assert keys & ['a', 'c'] == {'a'}
assert keys | ['c'] == {'a', 'b', 'c'}
assert items & [('b', 2)] == {('b', 2)}
assert ('a', 1) in items and ('a', 2) not in items
assert ('c', 1) not in items
assert 1 in values and 3 not in values
mapping['c'] = 3
mapping['a'] = 4
assert len(keys) == 3
assert list(values) == [4, 2, 3]
assert ('a', 4) in items and ('a', 1) not in items
assert mapping.setdefault('a', 99) == 4
assert mapping.setdefault('d', 5) == 5
assert mapping.pop('d') == 5
assert mapping.pop('missing', 7) == 7
try:
    mapping.pop('missing')
    assert False
except KeyError:
    pass
mapping.update([('a', 10), ('e', 6)], extra=7)
assert mapping['a'] == 10 and mapping['extra'] == 7
class Keyed:
    def keys(self):
        return ['keyed']
    def __getitem__(self, key):
        return 8
mapping.update(Keyed())
assert mapping['keyed'] == 8
mapping.update(Data([('nested', 9)]))
assert mapping['nested'] == 9
pair = mapping.popitem()
assert pair[0] not in mapping
mapping.clear()
assert not mapping
assert list(keys) == list(items) == list(values) == []
try:
    mapping.popitem()
    assert False
except KeyError:
    pass

first = Data([('a', 1), ('b', 2)])
second = Data([('b', 2), ('a', 1)])
assert first == second
assert first == {'a': 1, 'b': 2}
assert first != {'a': 1}
assert first != {'a': 2, 'b': 2}
assert Mapping.__eq__(first, []) is NotImplemented
try:
    hash(first)
    assert False
except TypeError:
    pass

class Equal:
    def __eq__(self, other):
        return True
shared = Equal()
assert Data([('a', shared)]) == {'a': Equal()}
assert shared in Data([('a', shared)]).values()
assert ('a', shared) in Data([('a', shared)]).items()
