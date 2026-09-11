# case: saved generator frames retain locals across suspension and completion
import sys
def generate():
    value = 1
    saved = sys._getframe()
    yield saved
    assert value == 2
    value = 3
generator = generate()
saved = next(generator)
assert saved.f_back is None
saved.f_locals['value'] = 2
assert next(generator, None) is None
assert saved.f_locals['value'] == 3
assert saved.f_back is None

# ---
# case: retained class frames preserve the original prepared namespace
import sys
class C:
    saved_frame = sys._getframe()
    value = 1
namespace = C.saved_frame.f_locals
namespace['value'] = 2
assert C.value == 1
C.value = 3
assert namespace['value'] == 2

# ---
# case: frame depth rejects invalid argument forms
import sys
for operation in [lambda: sys._getframe('x'), lambda: sys._getframe(0, 1), lambda: sys._getframe(depth=0)]:
    try:
        operation()
        assert False
    except TypeError:
        pass
try:
    sys._getframe(1 << 40)
    assert False
except OverflowError:
    pass

# ---
# case: frame identity and caller depth describe actual Python frames
import sys
import fixture
module_frame = sys._getframe()
assert type(module_frame).__name__ == 'frame'
assert module_frame is sys._getframe(0)
assert module_frame is sys._getframe(-1)
assert module_frame.f_back is None
assert module_frame.f_locals is fixture.__dict__
assert module_frame.f_globals is fixture.__dict__
assert module_frame.f_builtins['len'] is len
assert module_frame.f_lineno > 0
def outer():
    current = sys._getframe()
    def inner():
        inner_frame = sys._getframe()
        assert inner_frame.f_back is current
        assert sys._getframe(1) is current
        assert sys._getframe(2) is module_frame
        return inner_frame
    return current, inner()
a, b = outer()
assert b.f_back is a
assert a.f_back is module_frame
try:
    sys._getframe(100)
    assert False
except ValueError as error:
    assert str(error) == 'call stack is not deep enough'
class Depth:
    def __index__(self):
        return 0
assert sys._getframe(Depth()) is module_frame
try:
    module_frame.f_locals = {}
    assert False
except AttributeError:
    pass

# ---
# case: optimized frame locals write through fast slots and retain extra names
import sys
def run(argument):
    value = 1
    frame = sys._getframe()
    proxy = frame.f_locals
    assert type(proxy).__name__ == 'FrameLocalsProxy'
    assert type(proxy.keys()) is list
    assert type(proxy.items()) is list
    assert type(proxy.values()) is list
    assert proxy['argument'] == argument
    assert 'unbound' not in proxy
    proxy['value'] = 10
    assert value == 10
    value = 20
    assert proxy['value'] == 20
    proxy['unbound'] = 7
    assert unbound == 7
    unbound = 8
    proxy['extra'] = 99
    assert frame.f_locals['extra'] == 99
    try:
        extra
        assert False
    except NameError:
        pass
    del proxy['extra']
    assert 'extra' not in proxy
    try:
        del proxy['value']
        assert False
    except ValueError as error:
        assert str(error) == 'cannot remove local variables from FrameLocalsProxy'
    snapshot = proxy.copy()
    keys = proxy.keys()
    iterator = iter(proxy)
    value = 30
    proxy['added'] = 2
    assert snapshot['value'] == 20
    assert 'added' not in keys
    assert 'added' not in list(iterator)
    return frame
retained = run(3)
assert retained.f_locals['value'] == 30
retained.f_locals['value'] = 40
assert retained.f_locals['value'] == 40

# ---
# case: frame locals update shared cell and free variables
import sys
def outer():
    captured = 1
    frame = sys._getframe()
    def inner():
        nonlocal captured
        captured = 2
        proxy = sys._getframe().f_locals
        assert proxy['captured'] == 2
        proxy['captured'] = 3
        assert captured == 3
        return proxy
    proxy = inner()
    assert captured == 3
    frame.f_locals['captured'] = 4
    assert proxy['captured'] == 4
    return proxy
assert outer()['captured'] == 4

# ---
# case: class frame locals are the prepared dictionary
import sys
class C:
    namespace = sys._getframe().f_locals
    assert type(namespace) is dict
    namespace['value'] = 7
    assert value == 7
    del value
    assert 'value' not in namespace
    namespace['value'] = 8
assert C.value == 8

# ---
# case: frame locals methods retain mapping semantics
import sys
def run():
    value = 1
    proxy = sys._getframe().f_locals
    assert proxy.get('value') == 1
    assert proxy.get('absent', 2) == 2
    assert proxy.setdefault('value', 9) == 1
    assert proxy.setdefault('extra', 4) == 4
    assert proxy.pop('extra') == 4
    assert proxy.pop('absent', 5) == 5
    try:
        proxy.pop('value')
        assert False
    except ValueError:
        pass
    proxy.update({'value': 6, 'extra': 7})
    assert value == 6
    assert proxy['extra'] == 7
    assert len(proxy) == len(proxy.keys())
    assert bool(proxy)
run()
