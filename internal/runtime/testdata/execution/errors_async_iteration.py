# Runtime exception cases for asynchronous iteration.
# case: async for requires __aiter__
# error: TypeError
# message: "'async for' requires an object with __aiter__ method, got MissingAIter"
class MissingAIter:
    pass

async def use_missing_aiter():
    async for item in MissingAIter():
        pass

use_missing_aiter().send(None)

# ---
# case: __aiter__ result must implement __anext__
# error: TypeError
# message: "'async for' received an object from __aiter__ that does not implement __anext__: MissingANext"
class MissingANext:
    pass

class ReturnsMissingANext:
    def __aiter__(self):
        return MissingANext()

async def use_missing_anext():
    async for item in ReturnsMissingANext():
        pass

use_missing_anext().send(None)

# ---
# case: __anext__ result must be awaitable
# error: TypeError
# message: "'async for' received an invalid object from __anext__: int"
class NonAwaitableANext:
    def __aiter__(self):
        return self

    def __anext__(self):
        return 1

async def use_nonawaitable_anext():
    async for item in NonAwaitableANext():
        pass

use_nonawaitable_anext().send(None)

# ---
# case: async iterator errors propagate
# error: ValueError
# message: "async next failed"
class FailingANext:
    def __aiter__(self):
        return self

    async def __anext__(self):
        raise ValueError('async next failed')

async def use_failing_anext():
    async for item in FailingANext():
        pass

use_failing_anext().send(None)

# ---
# case: StopAsyncIteration from loop body is not exhaustion
# error: StopAsyncIteration
# message: "body stop"
class OneAsyncValue:
    def __init__(self):
        self.done = False

    def __aiter__(self):
        return self

    async def __anext__(self):
        if self.done:
            raise StopAsyncIteration
        self.done = True
        return 1

async def stop_in_async_body():
    async for item in OneAsyncValue():
        raise StopAsyncIteration('body stop')

stop_in_async_body().send(None)

# ---
# case: empty async for target remains unbound
# error: UnboundLocalError
# message: "cannot access local variable 'item' where it is not associated with a value"
class EmptyAsyncValues:
    def __aiter__(self):
        return self

    async def __anext__(self):
        raise StopAsyncIteration

async def read_empty_async_target():
    async for item in EmptyAsyncValues():
        pass
    return item

read_empty_async_target().send(None)
