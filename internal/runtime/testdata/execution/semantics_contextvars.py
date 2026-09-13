# case: context variable defaults and bindings preserve identity and absence
from _contextvars import ContextVar, Token
fallback = []
var = ContextVar('value', default=fallback)
other = ContextVar('value')
assert var.name == other.name == 'value'
assert var is not other
assert var.get() is fallback
assert var.get(None) is None
try:
    other.get()
    assert False
except LookupError as error:
    assert error.args == (other,)
value = []
token = var.set(value)
assert type(token) is Token
assert token.var is var
assert token.old_value is Token.MISSING
assert var.get(None) is value
second = var.set(None)
assert second.old_value is value
assert var.get() is None
assert var.reset(second) is None
assert var.get() is value
var.reset(token)
assert var.get() is fallback
assert repr(Token.MISSING) == '<Token.MISSING>'
assert {var: 1, other: 2}[var] == 1
assert len({var, other}) == 2
assert hash(var) == hash(var)
assert ContextVar[int].__origin__ is ContextVar
assert Token[int].__args__ == (int,)

# ---
# case: reset is single use but permits out of order tokens
from _contextvars import ContextVar, Token
var = ContextVar('value', default=0)
other = ContextVar('other')
first = var.set(1)
second = var.set(2)
try:
    other.reset(first)
    assert False
except ValueError:
    pass
assert var.get() == 2
var.reset(first)
assert var.get() == 0
var.reset(second)
assert var.get() == 1
try:
    other.reset(first)
    assert False
except RuntimeError:
    pass
assert first.old_value is Token.MISSING
assert second.old_value == 1
for value in [None, 1, var]:
    try:
        var.reset(value)
        assert False
    except TypeError:
        pass
try:
    hash(first)
    assert False
except TypeError:
    pass
try:
    Token()
    assert False
except RuntimeError as error:
    assert str(error) == 'Tokens can only be created by ContextVars'

# ---
# case: tokens restore values across nested context managers and exceptions
from _contextvars import ContextVar
var = ContextVar('value', default=0)
try:
    with var.set(1) as outer:
        assert outer.var is var
        with var.set(2):
            assert var.get() == 2
        assert var.get() == 1
        raise ValueError('body')
except ValueError as error:
    assert str(error) == 'body'
assert var.get() == 0
assert outer.__enter__() is outer
try:
    outer.__exit__(None, None, None)
    assert False
except RuntimeError:
    pass

# ---
# case: variable constructors and descriptors reject invalid arguments and mutation
from _contextvars import ContextVar, Token
for args, kwargs in [((), {}), ((1,), {}), (('name', 1), {}), ((), {'name': 'name'}), (('name',), {'unknown': 1})]:
    try:
        ContextVar(*args, **kwargs)
        assert False
    except TypeError:
        pass
class Name(str):
    pass
name = Name('subtype')
var = ContextVar(name)
assert var.name is name
assert ContextVar.get(var, 5) == 5
assert ContextVar.name.__get__(var) is name
token = ContextVar.set(var, 7)
assert Token.var.__get__(token) is var
assert Token.old_value.__get__(token) is Token.MISSING
for owner, attr in [(var, 'name'), (var, 'extra'), (token, 'var'), (token, 'old_value'), (token, 'extra')]:
    try:
        setattr(owner, attr, 1)
        assert False
    except AttributeError:
        pass
    try:
        delattr(owner, attr)
        assert False
    except AttributeError:
        pass
for method, args, kwargs in [(var.get, (), {'default': 1}), (var.set, (), {}), (var.reset, (), {}), (ContextVar.get, (1,), {}), (token.__enter__, (1,), {}), (token.__exit__, (), {})]:
    try:
        method(*args, **kwargs)
        assert False
    except TypeError:
        pass
for base in [ContextVar, Token]:
    try:
        class Child(base):
            pass
        assert False
    except TypeError:
        pass
    try:
        object.__new__(base)
        assert False
    except TypeError:
        pass
