# Runtime execution cases for basic generator suspension.
# case: lazy generator iteration and exhaustion
generator_state = 0

def produce():
    global generator_state
    generator_state = 1
    received = yield 4
    assert received is None
    yield 5
    generator_state = 2
    return 99

stream = produce()
assert generator_state == 0
seen = 0
for value in stream:
    assert generator_state == 1
    seen = seen * 10 + value
assert seen == 45
assert generator_state == 2
again = 0
for value in stream:
    again += value
assert again == 0
# ---
# case: generator arguments and closure capture
def factory(offset):
    def generate(items, scale=2):
        for item in items:
            yield item * scale + offset
    return generate

stream = factory(10)((1, 2), 3)
seen = 0
for value in stream:
    seen = seen * 100 + value
assert seen == 1316
# ---
# case: generator cleanup survives suspension
cleanup_state = 0
manager_state = 0

class Manager:
    def __enter__(self):
        global manager_state
        manager_state = 1
        return self

    def __exit__(self, kind, value, traceback):
        global manager_state
        manager_state += 100
        return False

def protected():
    global cleanup_state
    try:
        cleanup_state = 1
        yield 7
        cleanup_state = 2
    finally:
        cleanup_state = 3

def managed():
    global manager_state
    with Manager():
        yield 8
        manager_state += 10

for value in protected():
    assert value == 7
    assert cleanup_state == 1
assert cleanup_state == 3
for value in managed():
    assert value == 8
    assert manager_state == 1
assert manager_state == 111
# ---
# case: generator exception reaches caller handler
def fail_after_yield():
    yield 1
    raise ValueError('generator failed')

seen = 0
stream = fail_after_yield()
try:
    for value in stream:
        seen += value
except ValueError as error:
    caught = error
assert seen == 1
assert f'{caught!r}' == 'ValueError("generator failed")'
again = 0
for value in stream:
    again += value
assert again == 0
# ---
# case: next resumes generator and exposes return value
def exchange():
    received = yield 10
    assert received is None
    return 99

stream = exchange()
assert next(stream) == 10
try:
    next(stream)
except StopIteration as error:
    returned = error.value
assert returned == 99
try:
    next(stream)
except StopIteration as error:
    exhausted = error.value
assert exhausted is None
# ---
# case: next default handles generator exhaustion
def one_value():
    yield 4

stream = one_value()
assert next(stream, 40) == 4
assert next(stream, 40) == 40
assert next(stream, 50) == 50
# ---
# case: explicit StopIteration is transformed in generator
def invalid_stop():
    yield 1
    raise StopIteration('hidden')

stream = invalid_stop()
assert next(stream) == 1
try:
    next(stream)
except RuntimeError as error:
    transformed = error
assert f'{transformed!r}' == 'RuntimeError("generator raised StopIteration")'
assert transformed.__cause__ is transformed.__context__
assert f'{transformed.__cause__!r}' == 'StopIteration("hidden")'
# ---
# case: send supplies yield expression values
def exchange():
    first = yield 1
    second = yield first + 1
    return second

stream = exchange()
sender = stream.send
assert sender(None) == 1
assert sender(10) == 11
try:
    sender(20)
except StopIteration as error:
    returned = error.value
assert returned == 20
try:
    stream.send(None)
except StopIteration as error:
    exhausted = error.value
assert exhausted is None

fresh = exchange()
try:
    fresh.send(5)
except TypeError:
    rejected = True
assert rejected
assert fresh.send(None) == 1
# ---
# case: next and send share generator state
def communicate():
    received = yield 2
    yield received

stream = communicate()
assert next(stream) == 2
assert stream.send(30) == 30
assert next(stream, 40) == 40
# ---
# case: throw injects exceptions at suspended yield
def catcher():
    try:
        yield 'ready'
    except ValueError as error:
        injected = error
        yield error
    return injected

stream = catcher()
assert next(stream) == 'ready'
error = ValueError('boom')
assert stream.throw(error) is error
try:
    next(stream)
except StopIteration as stopped:
    returned = stopped.value
assert returned is error

stream = catcher()
next(stream)
legacy = stream.throw(ValueError, 'legacy')
assert f'{legacy!r}' == 'ValueError("legacy")'
stream = catcher()
next(stream)
legacy_none = stream.throw(ValueError, 'third', None)
assert f'{legacy_none!r}' == 'ValueError("third")'
# ---
# case: throw handles created and completed generators
throw_state = 0

def never_started():
    global throw_state
    throw_state = 1
    yield 1

stream = never_started()
injected = KeyError('created')
try:
    stream.throw(injected)
except KeyError as caught:
    created_error = caught
assert created_error is injected
assert throw_state == 0
assert next(stream, 9) == 9

stream = never_started()
assert next(stream) == 1
assert next(stream, None) is None
injected = ValueError('completed')
try:
    stream.throw(injected)
except ValueError as caught:
    completed_error = caught
assert completed_error is injected
# ---
# case: thrown StopIteration is transformed
def passive():
    yield 1

stream = passive()
next(stream)
injected = StopIteration('hidden')
try:
    stream.throw(injected)
except RuntimeError as error:
    transformed = error
assert transformed.__cause__ is injected
assert transformed.__context__ is injected
assert f'{transformed!r}' == 'RuntimeError("generator raised StopIteration")'
# ---
# case: close runs cleanup and returns generator value
close_state = 0

def cleanup():
    global close_state
    try:
        yield 1
    finally:
        close_state = 1

def returns_value():
    try:
        yield 2
    except GeneratorExit:
        return 9

stream = cleanup()
closer = stream.close
assert next(stream) == 1
assert closer() is None
assert close_state == 1
assert closer() is None
stream = returns_value()
assert next(stream) == 2
assert stream.close() == 9
assert stream.close() is None
# ---
# case: close skips new generator body
close_state = 0

def unopened():
    global close_state
    close_state = 1
    yield 1

stream = unopened()
assert stream.close() is None
assert close_state == 0
assert next(stream, 7) == 7
# ---
# case: yielding during close leaves generator suspended
def ignores_close():
    try:
        yield 1
    except GeneratorExit:
        yield 2

stream = ignores_close()
next(stream)
try:
    stream.close()
except RuntimeError as error:
    ignored = error
assert f'{ignored!r}' == 'RuntimeError("generator ignored GeneratorExit")'
assert next(stream, 8) == 8
# ---
# case: yield from delegates iterable values
def flatten(source):
    result = yield from source
    return result

stream = flatten((1, 2, 3))
seen = 0
for value in stream:
    seen = seen * 10 + value
assert seen == 123
stream = flatten([])
try:
    next(stream)
except StopIteration as error:
    empty_result = error.value
assert empty_result is None
# ---
# case: yield from forwards send and return value
def inner():
    received = yield 1
    yield received
    return 9

def outer():
    result = yield from inner()
    return result + 1

stream = outer()
assert next(stream) == 1
assert stream.send(7) == 7
try:
    next(stream)
except StopIteration as error:
    delegated_result = error.value
assert delegated_result == 10
# ---
# case: yield from propagates delegate exception to outer handler
def failing_inner():
    yield 1
    raise ValueError('delegate failed')

def recovering_outer():
    try:
        yield from failing_inner()
    except ValueError as error:
        caught = error
        yield 2
    return caught

stream = recovering_outer()
assert next(stream) == 1
assert next(stream) == 2
try:
    next(stream)
except StopIteration as error:
    caught = error.value
assert f'{caught!r}' == 'ValueError("delegate failed")'
# ---
# case: yield from forwards throw through nested generators
def throwing_inner():
    try:
        yield 1
    except ValueError as error:
        yield error
    return 6

def throwing_middle():
    result = yield from throwing_inner()
    return result + 1

def throwing_outer():
    result = yield from throwing_middle()
    return result + 1

stream = throwing_outer()
assert next(stream) == 1
injected = ValueError('forwarded')
assert stream.throw(injected) is injected
try:
    next(stream)
except StopIteration as error:
    throw_result = error.value
assert throw_result == 8
# ---
# case: yield from throws into outer when native iterator has no throw
def native_throw_outer():
    try:
        yield from (1, 2)
    except ValueError as error:
        yield error

stream = native_throw_outer()
assert next(stream) == 1
injected = ValueError('outer')
assert stream.throw(injected) is injected
assert next(stream, 7) == 7
# ---
# case: yield from closes nested generator delegates inside out
close_order = 0

def close_inner():
    global close_order
    try:
        yield 1
    finally:
        close_order = close_order * 10 + 1

def close_middle():
    global close_order
    try:
        yield from close_inner()
    finally:
        close_order = close_order * 10 + 2

def close_outer():
    global close_order
    try:
        yield from close_middle()
    finally:
        close_order = close_order * 10 + 3

stream = close_outer()
assert next(stream) == 1
assert stream.close() is None
assert close_order == 123
assert next(stream, 9) == 9
# ---
# case: yield from close skips native delegates
native_close_state = 0

def native_close():
    global native_close_state
    try:
        yield from (1, 2)
    finally:
        native_close_state = 1

stream = native_close()
assert next(stream) == 1
assert stream.close() is None
assert native_close_state == 1
assert next(stream, 9) == 9
# ---
# case: yield from preserves thrown GeneratorExit
throw_close_state = 0

def returning_delegate():
    global throw_close_state
    try:
        yield 1
    except GeneratorExit:
        throw_close_state = 1
        return 4

def throwing_outer():
    yield from returning_delegate()
    throw_close_state = 2

stream = throwing_outer()
assert next(stream) == 1
thrown = GeneratorExit('forwarded')
try:
    stream.throw(thrown)
except GeneratorExit as caught:
    forwarded_exit = caught
assert forwarded_exit is thrown
assert forwarded_exit.__context__ is None
assert throw_close_state == 1
assert next(stream, 9) == 9
# ---
# case: yield from reports a delegate that yields while closing
retained_delegate = None

def yielding_close_delegate():
    try:
        yield 1
    except GeneratorExit:
        yield 2

def yielding_close_outer():
    global retained_delegate
    retained_delegate = yielding_close_delegate()
    yield from retained_delegate

stream = yielding_close_outer()
assert next(stream) == 1
try:
    stream.close()
except RuntimeError as error:
    ignored_exit = error
assert f'{ignored_exit!r}' == 'RuntimeError("generator ignored GeneratorExit")'
assert next(stream, 9) == 9
assert next(retained_delegate, 8) == 8
