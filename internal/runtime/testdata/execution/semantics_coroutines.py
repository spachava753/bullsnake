# Runtime execution cases for coroutine and asynchronous-generator creation.
# case: coroutine calls are lazy and closable
events = 0
async def compute():
    global events
    events = 1
    return 7
coroutine = compute()
assert events == 0
assert f'{coroutine!r}' == "<coroutine object compute>", "coroutine"
coroutine.close()
assert events == 0
# ---
# case: asynchronous generators are distinct lazy values
async def stream():
    yield 1
generator = stream()
assert f'{generator!r}' == "<async_generator object stream>", "generator"
# ---
# case: asyncio runner and context isolation
import asyncio
from _contextvars import ContextVar, copy_context

marker = ContextVar('marker', default=())
context = copy_context()

async def update_marker():
    marker.set(marker.get() + ('async',))
    await asyncio.sleep(0)
    return marker.get()

runner = asyncio.Runner(debug=True)
assert runner.run(update_marker(), context=context) == ('async',)
assert marker.get() == ()
runner.close()
# ---
# case: asyncio task cancellation
import asyncio

cancelled = False

async def pending():
    global cancelled
    try:
        await asyncio.sleep(1)
    except asyncio.CancelledError:
        cancelled = True
        raise

async def start_task():
    asyncio.create_task(pending())

asyncio.Runner().run(start_task())
assert cancelled
# ---
# case: asynchronous iteration and comprehensions
import asyncio

class AsyncRange:
    def __init__(self, stop):
        self.current = 0
        self.stop = stop

    def __aiter__(self):
        return self

    async def __anext__(self):
        if self.current >= self.stop:
            raise StopAsyncIteration
        value = self.current
        self.current += 1
        return value

async def collect():
    visited = []
    async for value in AsyncRange(3):
        visited.append(value)
    doubled = [value * 2 async for value in AsyncRange(4) if value]
    return visited, doubled

assert asyncio.run(collect()) == ([0, 1, 2], [2, 4, 6])
