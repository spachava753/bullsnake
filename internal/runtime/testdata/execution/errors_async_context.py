# Runtime exception cases for asynchronous context managers.
# case: async context manager missing exit
# error: TypeError
# message: "'MissingAsyncExit' object does not support the asynchronous context manager protocol (missed __aexit__ method)"
class MissingAsyncExit:
    async def __aenter__(self):
        return self

async def use_missing_async_exit():
    async with MissingAsyncExit():
        pass

use_missing_async_exit().send(None)

# ---
# case: async context manager missing enter
# error: TypeError
# message: "'MissingAsyncEnter' object does not support the asynchronous context manager protocol (missed __aenter__ method)"
class MissingAsyncEnter:
    async def __aexit__(self, kind, value, traceback):
        return False

async def use_missing_async_enter():
    async with MissingAsyncEnter():
        pass

use_missing_async_enter().send(None)

# ---
# case: async enter result must be awaitable
# error: TypeError
# message: "'async with' received an object from __aenter__ that does not implement __await__: int"
class NonAwaitableAsyncEnter:
    def __aenter__(self):
        return 1

    async def __aexit__(self, kind, value, traceback):
        return False

async def use_nonawaitable_async_enter():
    async with NonAwaitableAsyncEnter():
        pass

use_nonawaitable_async_enter().send(None)

# ---
# case: async exit result must be awaitable
# error: TypeError
# message: "'async with' received an object from __aexit__ that does not implement __await__: bool"
class NonAwaitableAsyncExit:
    async def __aenter__(self):
        return self

    def __aexit__(self, kind, value, traceback):
        return False

async def use_nonawaitable_async_exit():
    async with NonAwaitableAsyncExit():
        pass

use_nonawaitable_async_exit().send(None)

# ---
# case: async context manager does not suppress exception
# error: ValueError
# message: "visible async error"
class DoesNotSuppressAsync:
    async def __aenter__(self):
        return self

    async def __aexit__(self, kind, value, traceback):
        return False

async def use_nonsuppressing_async_manager():
    async with DoesNotSuppressAsync():
        raise ValueError('visible async error')

use_nonsuppressing_async_manager().send(None)

# ---
# case: async exit replaces normal completion
# error: RuntimeError
# message: "async exit failed"
class FailingAsyncExit:
    async def __aenter__(self):
        return self

    async def __aexit__(self, kind, value, traceback):
        raise RuntimeError('async exit failed')

async def use_failing_async_exit():
    async with FailingAsyncExit():
        pass

use_failing_async_exit().send(None)
