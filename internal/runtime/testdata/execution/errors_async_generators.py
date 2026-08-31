# Runtime exception cases for asynchronous generators.
# case: async generator failures propagate through async for
# error: ValueError
# message: "async generator failed"
async def failing_async_values():
    yield 1
    raise ValueError('async generator failed')

async def consume_failing_async_values():
    async for value in failing_async_values():
        pass

consume_failing_async_values().send(None)

# ---
# case: escaping StopAsyncIteration is transformed
# error: RuntimeError
# message: "async generator raised StopAsyncIteration"
async def invalid_async_stop():
    raise StopAsyncIteration('manual stop')
    yield 1

async def consume_invalid_async_stop():
    async for value in invalid_async_stop():
        pass

consume_invalid_async_stop().send(None)

# ---
# case: async generators are not synchronous iterables
# error: TypeError
# message: "'async_generator' object is not iterable"
async def asynchronous_only():
    yield 1

for value in asynchronous_only():
    pass

# ---
# case: async generators are not awaitable
# error: TypeError
# message: "'async_generator' object can't be awaited"
async def not_a_coroutine():
    yield 1

async def await_async_generator():
    await not_a_coroutine()

await_async_generator().send(None)

# ---
# case: next rejects async generators
# error: TypeError
# message: "'async_generator' object is not an iterator"
async def not_a_sync_iterator():
    yield 1

next(not_a_sync_iterator())

# ---
# case: async generator initializers are rejected
# error: TypeError
# message: "__init__() should return None, not 'async_generator'"
class AsyncGeneratorInitializer:
    async def __init__(self):
        yield 1

AsyncGeneratorInitializer()
