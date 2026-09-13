# case: dict update accepts real mappings and applies keywords last
class Source:
    def keys(self):
        return ['first', 'second']
    def __getitem__(self, key):
        return key
mapping = {'first': 'old'}
assert mapping.update(Source(), first='keyword', third=3) is None
assert list(mapping.items()) == [('first', 'keyword'), ('second', 'second'), ('third', 3)]
class Attributes:
    field = 42
mapping = {}
mapping.update(Attributes.__dict__)
assert mapping['field'] == 42
assert dict(Source()) == {'first': 'first', 'second': 'second'}
class Subtype(dict):
    pass
subtype = Subtype(Source(), extra=5)
assert subtype == {'first': 'first', 'second': 'second', 'extra': 5}
assert dict(subtype) == subtype

# ---
# case: mapping keys are captured before value lookup and inserts are incremental
calls = []
target = {}
class Source:
    def keys(self):
        calls.append('keys')
        yield 'a'
        calls.append('keys finished')
        yield 'b'
    def __getitem__(self, key):
        calls.append(key)
        if key == 'b':
            assert target == {'a': 1}
            raise ValueError('value failed')
        return 1
try:
    target.update(Source(), last=3)
    assert False
except ValueError as error:
    assert str(error) == 'value failed'
assert calls == ['keys', 'keys finished', 'a', 'b']
assert target == {'a': 1}

# ---
# case: iterable-pair updates consume inner iterables and retain partial writes
target = {}
def pair():
    yield 'a'
    yield 1
def source():
    yield pair()
    assert target == {'a': 1}
    yield ['b', 2]
    raise ValueError('source failed')
try:
    target.update(source(), last=3)
    assert False
except ValueError as error:
    assert str(error) == 'source failed'
assert target == {'a': 1, 'b': 2}
assert dict([pair(), ('b', 2)]) == target
assert dict.__init__(target, [('c', 3)]) is None
assert target == {'a': 1, 'b': 2, 'c': 3}

# ---
# case: invalid update sequence elements report position and preserve earlier writes
target = {}
try:
    target.update([('a', 1), ('bad',)])
    assert False
except ValueError as error:
    assert str(error) == 'dictionary update sequence element #1 has length 1; 2 is required'
assert target == {'a': 1}
try:
    target.update([42])
    assert False
except TypeError as error:
    assert str(error) == 'cannot convert dictionary update sequence element #0 to a sequence'

# ---
# case: dict subtype source fast path depends on its iteration slot
calls = []
class NativeIteration(dict):
    def __getitem__(self, key):
        calls.append('get')
        return 99
    def keys(self):
        calls.append('keys')
        return ['a']
source = NativeIteration({'a': 1})
assert dict(source) == {'a': 1}
assert calls == []
class CustomIteration(NativeIteration):
    def __iter__(self):
        return iter(['a'])
source = CustomIteration({'a': 1})
assert dict(source) == {'a': 99}
assert calls == ['keys', 'get']

# ---
# case: lookup absence alone enables iterable fallback
class Source:
    @property
    def keys(self):
        raise AttributeError('no mapping protocol')
    def __iter__(self):
        yield ('a', 1)
assert dict(Source()) == {'a': 1}
class Broken:
    def keys(self):
        raise AttributeError('method failed')
    def __iter__(self):
        assert False
        yield None
try:
    dict(Broken())
    assert False
except AttributeError as error:
    assert str(error) == 'method failed'

# ---
# case: inner iterator TypeError is not rewritten as conversion failure
def broken_pair():
    yield 'a'
    raise TypeError('inner iterator failed')
try:
    dict([broken_pair()])
    assert False
except TypeError as error:
    assert str(error) == 'inner iterator failed'

# ---
# case: many immediate dictionary input steps use bounded native stack depth
mapping = {}
mapping.update([('same', item) for item in range(10000)])
assert mapping == {'same': 9999}
