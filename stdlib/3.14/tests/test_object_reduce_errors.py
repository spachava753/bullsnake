class Empty:
    pass
empty = Empty()
# Exercise the protocol-zero helper on a cold source-module cache.
r = object.__reduce__(empty)
import copyreg
assert r == (copyreg._reconstructor, (Empty, object, None))
assert type(r[0](*r[1])) is Empty

class Bad:
    result = None
    def __getnewargs_ex__(self):
        return self.result
bad = Bad()
for result, error_type, message in (
    (None, TypeError, "__getnewargs_ex__ should return a tuple, not 'NoneType'"),
    ((), ValueError, '__getnewargs_ex__ should return a tuple of length 2, not 0'),
    (([], {}), TypeError, "first item of the tuple returned by __getnewargs_ex__ must be a tuple, not 'list'"),
    (((), []), TypeError, "second item of the tuple returned by __getnewargs_ex__ must be a dict, not 'list'"),
):
    bad.result = result
    try:
        bad.__reduce_ex__(4)
    except error_type as error:
        assert str(error) == message
    else:
        assert False
bad.result = ((), {})
assert bad.__reduce_ex__(4)[1] == (Bad,)
class OldBad:
    def __getnewargs__(self):
        return None
try:
    OldBad().__reduce_ex__(4)
except TypeError as error:
    assert str(error) == "__getnewargs__ should return a tuple, not 'NoneType'"
else:
    assert False

calls = []
class Hook:
    def __get__(self, instance, cls):
        calls.append((instance, cls))
        return lambda: ((), {'x': 1})
class DescriptorArguments:
    __getnewargs_ex__ = Hook()
value = DescriptorArguments()
r = value.__reduce_ex__(4)
assert calls == [(value, DescriptorArguments)]
assert r[:2] == (copyreg.__newobj_ex__, (DescriptorArguments, (), {'x': 1}))

class Failing:
    @property
    def __reduce__(self):
        raise ValueError('reduce lookup')
try:
    Failing().__reduce_ex__(4)
except ValueError as error:
    assert str(error) == 'reduce lookup'
else:
    assert False
class Missing:
    @property
    def __reduce__(self):
        raise AttributeError('no reduce')
assert Missing().__reduce_ex__(4)[0] is copyreg.__newobj__
class NoCall:
    __reduce__ = None
try:
    NoCall().__reduce_ex__(4)
except TypeError:
    pass
else:
    assert False

class State:
    def __getnewargs__(self):
        calls.append('arguments')
        return ()
    def __getstate__(self):
        calls.append('state')
        raise ValueError('state failed')
state = State()
calls = []
try:
    state.__reduce_ex__(4)
except ValueError as error:
    assert str(error) == 'state failed'
else:
    assert False
assert calls == ['arguments', 'state']
state.__getstate__ = lambda: 'replaced'
assert state.__reduce_ex__(4)[2] == 'replaced'

class KeywordDictionary(dict):
    pass
keywords = KeywordDictionary(x=3)
args = (1,)
class Identity:
    def __getnewargs_ex__(self):
        return args, keywords
r = Identity().__reduce_ex__(4)
assert r[1][1] is args
assert r[1][2] is keywords

class Mapping(dict):
    def __getstate__(self):
        calls.append('state')
        return None
    def items(self):
        calls.append('items')
        raise ValueError('items failed')
calls = []
try:
    Mapping().__reduce_ex__(4)
except ValueError as error:
    assert str(error) == 'items failed'
else:
    assert False
assert calls == ['state', 'items']

# Native storage without a supported reduction must not silently disappear.
from io import BytesIO
try:
    object.__reduce_ex__(BytesIO(b'payload'), 4)
except TypeError:
    pass
else:
    assert False
