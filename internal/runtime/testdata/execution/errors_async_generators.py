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

# ---
# case: first async generator asend value must be None
# error: TypeError
# message: "can't send non-None value to a just-started async generator"
async def new_async_sender():
    yield 1

async def start_async_sender_with_value():
    await new_async_sender().asend(4)

start_async_sender_with_value().send(None)

# ---
# case: async generator next awaitables are one shot
# error: RuntimeError
# message: "cannot reuse already awaited __anext__()/asend()"
async def reusable_async_sender():
    yield 1

async def reuse_async_sender_awaitable():
    pending = reusable_async_sender().asend(None)
    await pending
    await pending

reuse_async_sender_awaitable().send(None)

# ---
# case: uncaught async generator athrow exceptions propagate
# error: ValueError
# message: "uncaught async throw"
async def uncaught_async_throw_target():
    yield 1

async def raise_into_async_generator():
    stream = uncaught_async_throw_target()
    await stream.asend(None)
    await stream.athrow(ValueError('uncaught async throw'))

raise_into_async_generator().send(None)

# ---
# case: async generator athrow awaitables are one shot
# error: RuntimeError
# message: "cannot reuse already awaited aclose()/athrow()"
async def reusable_async_throw_target():
    try:
        yield 1
    except ValueError:
        yield 2

async def reuse_async_throw_awaitable():
    stream = reusable_async_throw_target()
    await stream.asend(None)
    pending = stream.athrow(ValueError)
    await pending
    await pending

reuse_async_throw_awaitable().send(None)

# ---
# case: async generator athrow validates exception values
# error: TypeError
# message: "exceptions must be classes or instances deriving from BaseException, not int"
async def invalid_async_throw_target():
    yield 1

async def throw_invalid_async_value():
    stream = invalid_async_throw_target()
    await stream.asend(None)
    await stream.athrow(1)

throw_invalid_async_value().send(None)

# ---
# case: yielding while closing an async generator is rejected
# error: RuntimeError
# message: "async generator ignored GeneratorExit"
async def yielding_async_close():
    try:
        yield 1
    except GeneratorExit:
        yield 2

async def close_yielding_async_generator():
    stream = yielding_async_close()
    await stream.asend(None)
    await stream.aclose()

close_yielding_async_generator().send(None)

# ---
# case: replacement async generator close exceptions propagate
# error: ValueError
# message: "async close failed"
async def failing_async_close():
    try:
        yield 1
    finally:
        raise ValueError('async close failed')

async def close_failing_async_generator():
    stream = failing_async_close()
    await stream.asend(None)
    await stream.aclose()

close_failing_async_generator().send(None)

# ---
# case: async generator aclose awaitables are one shot
# error: RuntimeError
# message: "cannot reuse already awaited aclose()/athrow()"
async def reusable_async_close():
    yield 1

async def reuse_async_close_awaitable():
    stream = reusable_async_close()
    await stream.asend(None)
    pending = stream.aclose()
    await pending
    await pending

reuse_async_close_awaitable().send(None)
