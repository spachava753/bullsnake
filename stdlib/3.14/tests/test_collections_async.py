# Project-owned behavior tests against unchanged CPython 3.14.7 _collections_abc.
from _collections_abc import Awaitable, Coroutine, Generator, AsyncIterable, AsyncIterator, AsyncGenerator

def run(awaitable):
    async def wait():
        return await awaitable
    try:
        wait().send(None)
        assert False
    except StopIteration as result:
        return result.value

class Sendable(Generator):
    def send(self, value):
        if value is None:
            return 1
        return super().send(value)
    def throw(self, typ, val=None, tb=None):
        return super().throw(typ, val, tb)
value = Sendable()
assert Generator.__subclasshook__(Sendable) is True
assert iter(value) is value
assert next(value) == 1
assert value.close() is None
try:
    value.send(2)
    assert False
except StopIteration:
    pass
try:
    value.throw(ValueError('failure'))
    assert False
except ValueError as error:
    assert str(error) == 'failure'
class IgnoresClose(Sendable):
    def throw(self, typ, val=None, tb=None):
        return 1
try:
    IgnoresClose().close()
    assert False
except RuntimeError as error:
    assert str(error) == 'generator ignored GeneratorExit'

class Task(Coroutine):
    def __await__(self):
        return iter([])
    def send(self, value):
        return super().send(value)
    def throw(self, typ, val=None, tb=None):
        return super().throw(typ, val, tb)
assert isinstance(Task(), Awaitable)
assert Coroutine.__subclasshook__(Task) is True
assert list(Awaitable.__await__(None)) == [None]
assert list(Task().__await__()) == []
assert Task().close() is None
try:
    Task().send(None)
    assert False
except StopIteration:
    pass
class IgnoringTask(Task):
    def throw(self, typ, val=None, tb=None):
        return 1
try:
    IgnoringTask().close()
    assert False
except RuntimeError as error:
    assert str(error) == 'coroutine ignored GeneratorExit'

class AsyncValues(AsyncGenerator):
    def __init__(self):
        self.finished = False
    async def asend(self, value):
        if self.finished:
            return await super().asend(value)
        self.finished = True
        return value
    async def athrow(self, typ, val=None, tb=None):
        return await super().athrow(typ, val, tb)
value = AsyncValues()
assert AsyncGenerator.__subclasshook__(AsyncValues) is True
assert isinstance(value, AsyncIterable)
assert isinstance(value, AsyncIterator)
assert value.__aiter__() is value
assert run(value.__anext__()) is None
try:
    run(value.__anext__())
    assert False
except StopAsyncIteration:
    pass
assert run(value.aclose()) is None
try:
    run(value.athrow(ValueError('async failure')))
    assert False
except ValueError as error:
    assert str(error) == 'async failure'
class IgnoringAsync(AsyncValues):
    async def athrow(self, typ, val=None, tb=None):
        return 1
try:
    run(IgnoringAsync().aclose())
    assert False
except RuntimeError as error:
    assert str(error) == 'asynchronous generator ignored GeneratorExit'

async def native_coroutine():
    return 7
native = native_coroutine()
assert isinstance(native, Awaitable)
assert isinstance(native, Coroutine)
assert run(native) == 7
async def native_async_generator():
    yield 8
native = native_async_generator()
assert isinstance(native, AsyncIterable)
assert isinstance(native, AsyncIterator)
assert isinstance(native, AsyncGenerator)
assert run(native.__anext__()) == 8
assert run(native.aclose()) is None
native = (x for x in [1])
assert isinstance(native, Generator)
assert next(native) == 1
native.close()

for cls in [Awaitable, Coroutine, Generator, AsyncIterable, AsyncIterator, AsyncGenerator]:
    try:
        cls()
        assert False
    except TypeError:
        pass
    assert cls[int].__origin__ is cls
