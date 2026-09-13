# case: traceback links retain real frames and stable exception identities
import sys
frames = []
def leaf():
    local = 'retained'
    frames.append(sys._getframe())
    raise ValueError('leaf')
assert ValueError().__traceback__ is None
try:
    leaf()
except ValueError as error:
    saved = error
    traceback = error.__traceback__
    assert traceback is error.__traceback__
    assert type(traceback).__name__ == 'traceback'
    assert traceback.tb_frame is sys._getframe()
    inner = traceback.tb_next
    assert inner.tb_next is None
    assert inner.tb_frame is frames[0]
    assert inner.tb_frame.f_code is leaf.__code__
    assert inner.tb_lineno == leaf.__code__.co_firstlineno + 3
    assert inner.tb_lasti >= 0
    assert inner.tb_frame.f_locals['local'] == 'retained'
assert saved.__traceback__ is traceback
assert getattr(traceback, 'tb_next') is inner
assert inner.tb_frame.f_locals['local'] == 'retained'

# ---
# case: traceback attachment clearing and reraising preserve old links
try:
    raise ValueError('first')
except ValueError as error:
    saved = error
    original = error.__traceback__
try:
    raise saved
except ValueError as error:
    assert error.__traceback__.tb_next is original
    assert original.tb_next is None
    attached = TypeError('second').with_traceback(original)
    assert attached.__traceback__ is original
    assert attached.with_traceback(None) is attached
    assert attached.__traceback__ is None
    attached.__traceback__ = original
    assert attached.__traceback__ is original
    setattr(attached, '__traceback__', None)
    assert attached.__traceback__ is None
try:
    raise saved
except ValueError as error:
    error.__traceback__ = None
    try:
        raise
    except ValueError as reraised:
        assert reraised.__traceback__ is None
for value in [1, 'bad']:
    try:
        attached.__traceback__ = value
        assert False
    except TypeError as error:
        assert str(error) == '__traceback__ must be a traceback or None'
try:
    del attached.__traceback__
    assert False
except TypeError as error:
    assert str(error) == '__traceback__ may not be deleted'

# ---
# case: traceback next mutation is shared and rejects loops
try:
    raise ValueError('one')
except ValueError as error:
    first = error.__traceback__
try:
    raise ValueError('two')
except ValueError as error:
    second = error.__traceback__
first.tb_next = second
assert first.tb_next is second
try:
    second.tb_next = first
    assert False
except ValueError as error:
    assert str(error) == 'traceback loop detected'
assert second.tb_next is None
first.tb_next = None
assert first.tb_next is None
try:
    first.tb_next = 1
    assert False
except TypeError:
    pass
for name in ['tb_frame', 'tb_lineno', 'tb_lasti']:
    try:
        setattr(first, name, 1)
        assert False
    except AttributeError:
        pass
try:
    del first.tb_next
    assert False
except TypeError as error:
    assert str(error) == "can't delete tb_next attribute"

# ---
# case: exception subgroups share traceback links
original = ExceptionGroup('group', [ValueError('value'), TypeError('type')])
try:
    raise original
except* ValueError as part:
    assert part.__traceback__ is original.__traceback__
except* TypeError as part:
    assert part.__traceback__ is original.__traceback__
