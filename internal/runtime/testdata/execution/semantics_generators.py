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
