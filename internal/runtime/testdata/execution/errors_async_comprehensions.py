# Runtime exception cases for asynchronous comprehensions.
# case: async comprehension targets do not leak
# error: NameError
# message: "name 'item' is not defined"
class EmptyAsyncComprehension:
    def __aiter__(self):
        return self

    async def __anext__(self):
        raise StopAsyncIteration

async def read_async_comprehension_target():
    [item async for item in EmptyAsyncComprehension()]
    return item

read_async_comprehension_target().send(None)

# ---
# case: async comprehension iterator errors propagate
# error: ValueError
# message: "async comprehension failed"
class FailingAsyncComprehension:
    def __aiter__(self):
        return self

    async def __anext__(self):
        raise ValueError('async comprehension failed')

async def build_failing_async_comprehension():
    return [item async for item in FailingAsyncComprehension()]

build_failing_async_comprehension().send(None)
