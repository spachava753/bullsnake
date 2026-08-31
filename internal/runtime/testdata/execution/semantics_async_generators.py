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
