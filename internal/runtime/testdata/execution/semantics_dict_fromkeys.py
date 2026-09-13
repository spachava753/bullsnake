# case: dict fromkeys has a real classmethod descriptor and class binding
descriptor = dict.__dict__['fromkeys']
assert type(descriptor).__name__ == 'classmethod_descriptor'
assert descriptor.__objclass__ is dict
assert descriptor.__name__ == 'fromkeys'
assert dict.fromkeys.__self__ is dict
assert {}.fromkeys.__self__ is dict
assert type(dict.fromkeys) is type(len)
assert descriptor.__get__(None, dict).__self__ is dict
assert descriptor.__get__({}).__self__ is dict
assert descriptor(dict, ['a']) == {'a': None}
assert descriptor.__get__(None, dict)(['a'], 1) == {'a': 1}
class Wrong:
    fromkeys = descriptor
try:
    Wrong.fromkeys
    assert False
except TypeError:
    pass
try:
    Wrong().fromkeys
    assert False
except TypeError:
    pass
for args in [(), (int, []), ({}, [])]:
    try:
        descriptor(*args)
        assert False
    except TypeError:
        pass
for instance, owner in [(None, None), ({}, int), (None, object)]:
    try:
        descriptor.__get__(instance, owner)
        assert False
    except TypeError:
        pass

# ---
# case: fromkeys preserves order shared values and fresh dictionary identity
shared = []
result = dict.fromkeys(['b', 'a', 'b'], shared)
assert list(result) == ['b', 'a']
assert result['a'] is shared and result['b'] is shared
shared.append(1)
assert result == {'b': [1], 'a': [1]}
assert dict.fromkeys('aba') == {'a': None, 'b': None}
assert dict.fromkeys([]) == {}
assert dict.fromkeys([]) is not dict.fromkeys([])
source = {'old': 1}
assert source.fromkeys(['new'], 2) == {'new': 2}
assert source == {'old': 1}
assert len(dict.fromkeys(range(1000))) == 1000

# ---
# case: fromkeys resumes iteration and stops at the first invalid key
calls = []
def keys():
    calls.append(1)
    yield 'a'
    calls.append(2)
    yield []
    calls.append(3)
try:
    dict.fromkeys(keys())
    assert False
except TypeError:
    pass
assert calls == [1, 2]
def broken():
    yield 1
    raise ValueError('iterator failed')
try:
    dict.fromkeys(broken())
    assert False
except ValueError as error:
    assert str(error) == 'iterator failed'
for args in [(), ([], 1, 2), (1,)]:
    try:
        dict.fromkeys(*args)
        assert False
    except TypeError:
        pass
try:
    dict.fromkeys([], value=1)
    assert False
except TypeError:
    pass
