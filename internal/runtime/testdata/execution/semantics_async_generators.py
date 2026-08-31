# Runtime execution cases for asynchronous generators.
# case: async generators are lazy and drive async for
activity = 0

async def immediate(value):
    return value

async def async_values(start):
    global activity
    activity += 1
    try:
        yield await immediate(start)
        yield start + 1
    finally:
        activity += 10

async def collect_async_values():
    stream = async_values(4)
    assert activity == 0
    total = 0
    async for value in stream:
        total = total * 10 + value
    async for value in stream:
        total = 99
    return total

collecting = collect_async_values()
try:
    collecting.send(None)
except StopIteration as stopped:
    assert stopped.value == 45
else:
    assert False
assert activity == 11

# ---
# case: async generators retain arguments and closure cells
def make_async_values(offset):
    async def values(first, second):
        yield offset + first
        yield offset + second
    return values

async def collect_closed_async_values():
    build = make_async_values(10)
    total = 0
    async for value in build(2, 3):
        total += value
    return total

closed_collection = collect_closed_async_values()
try:
    closed_collection.send(None)
except StopIteration as stopped:
    assert stopped.value == 25
else:
    assert False

# ---
# case: async generator iterator methods expose awaitable next values
async def direct_async_values():
    yield 7
    yield 8

async def read_direct_async_values():
    stream = direct_async_values()
    assert stream.__aiter__() is stream
    first = await stream.__anext__()
    second = await stream.__anext__()
    exhausted = False
    try:
        await stream.__anext__()
    except StopAsyncIteration:
        exhausted = True
    return (first, second, exhausted)

direct_reading = read_direct_async_values()
try:
    direct_reading.send(None)
except StopIteration as stopped:
    assert stopped.value == (7, 8, True)
else:
    assert False

# ---
# case: async generator asend supplies suspended yield results
asend_activity = 0

async def exchange_async_values():
    global asend_activity
    asend_activity += 1
    first = yield 1
    second = yield first
    yield second

async def drive_async_sends():
    stream = exchange_async_values()
    pending = stream.asend(None)
    assert asend_activity == 0
    first = await pending
    second = await stream.asend(20)
    third = await stream.asend(30)
    exhausted = False
    try:
        await stream.asend(None)
    except StopAsyncIteration:
        exhausted = True
    return (first, second, third, exhausted)

sending = drive_async_sends()
try:
    sending.send(None)
except StopIteration as stopped:
    assert stopped.value == (1, 20, 30, True)
else:
    assert False
assert asend_activity == 1

# ---
# case: failed first asend closes only its awaitable
async def restartable_async_sender():
    yield 5

async def recover_from_invalid_first_asend():
    stream = restartable_async_sender()
    rejected = stream.asend(4)
    first_failed = False
    try:
        await rejected
    except TypeError:
        first_failed = True
    reuse_failed = False
    try:
        await rejected
    except RuntimeError:
        reuse_failed = True
    value = await stream.asend(None)
    return (first_failed, reuse_failed, value)

recovering = recover_from_invalid_first_asend()
try:
    recovering.send(None)
except StopIteration as stopped:
    assert stopped.value == (True, True, 5)
else:
    assert False

# ---
# case: async generator athrow injects exceptions at yield
class AsyncSignal(Exception):
    pass

async def catch_async_signal():
    try:
        yield 1
    except AsyncSignal as caught:
        yield caught

async def drive_async_throw():
    stream = catch_async_signal()
    first = await stream.asend(None)
    signal = AsyncSignal('injected')
    caught = await stream.athrow(signal)
    exhausted = False
    try:
        await stream.asend(None)
    except StopAsyncIteration:
        exhausted = True
    return (first, caught is signal, exhausted)

throwing = drive_async_throw()
try:
    throwing.send(None)
except StopIteration as stopped:
    assert stopped.value == (1, True, True)
else:
    assert False

# ---
# case: athrow into a new async generator skips its body
new_throw_activity = 0

async def untouched_async_generator():
    global new_throw_activity
    new_throw_activity += 1
    yield 1

async def throw_into_new_async_generator():
    stream = untouched_async_generator()
    injected = False
    try:
        await stream.athrow(ValueError('new throw'))
    except ValueError:
        injected = True
    exhausted = False
    try:
        await stream.__anext__()
    except StopAsyncIteration:
        exhausted = True
    return (injected, exhausted)

new_throwing = throw_into_new_async_generator()
try:
    new_throwing.send(None)
except StopIteration as stopped:
    assert stopped.value == (True, True)
else:
    assert False
assert new_throw_activity == 0

# ---
# case: async generator aclose runs cleanup and is repeatable
close_activity = 0

async def close_pause(value):
    return value

async def closable_async_values():
    global close_activity
    try:
        yield 1
        yield 2
    finally:
        await close_pause(None)
        close_activity += 1

async def close_async_values():
    stream = closable_async_values()
    first = await stream.asend(None)
    closed = await stream.aclose()
    closed_again = await stream.aclose()
    exhausted = False
    try:
        await stream.__anext__()
    except StopAsyncIteration:
        exhausted = True
    return (first, closed is None, closed_again is None, exhausted)

closing = close_async_values()
try:
    closing.send(None)
except StopIteration as stopped:
    assert stopped.value == (1, True, True, True)
else:
    assert False
assert close_activity == 1

# ---
# case: aclose on a new async generator skips its body
new_close_activity = 0

async def unopened_async_values():
    global new_close_activity
    new_close_activity += 1
    yield 1

async def close_new_async_values():
    stream = unopened_async_values()
    result = await stream.aclose()
    return result is None

new_closing = close_new_async_values()
try:
    new_closing.send(None)
except StopIteration as stopped:
    assert stopped.value is True
else:
    assert False
assert new_close_activity == 0
