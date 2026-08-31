# Runtime execution cases for asynchronous iteration.
# case: async for awaits values, destructures targets, and runs else
class AsyncPairs:
    def __init__(self, values, stop):
        self.values = values
        self.stop = stop
        self.index = 0

    def __aiter__(self):
        return self

    async def __anext__(self):
        if self.index >= self.stop:
            raise StopAsyncIteration
        value = self.values[self.index]
        self.index += 1
        return value

async def collect_async_pairs(source):
    total = 0
    async for left, right in source:
        if left == 3:
            continue
        total += left + right
    else:
        total += 100
    return total

pairs = AsyncPairs(((1, 2), (3, 4)), 2)
pairs.__aiter__ = None
collecting = collect_async_pairs(pairs)
try:
    collecting.send(None)
except StopIteration as stopped:
    assert stopped.value == 103
else:
    assert False
assert pairs.index == 2

# ---
# case: async for break skips else
class AsyncNumbers:
    def __init__(self, stop):
        self.stop = stop
        self.index = 0

    def __aiter__(self):
        return self

    async def __anext__(self):
        if self.index >= self.stop:
            raise StopAsyncIteration
        self.index += 1
        return self.index

async def break_async_loop(source):
    result = 0
    async for item in source:
        result = item
        break
    else:
        result = 99
    return result

breaking = break_async_loop(AsyncNumbers(3))
try:
    breaking.send(None)
except StopIteration as stopped:
    assert stopped.value == 1
else:
    assert False

# ---
# case: async for empty iteration runs else without entering body
class EmptyAsyncIterator:
    def __aiter__(self):
        return self

    async def __anext__(self):
        raise StopAsyncIteration

async def consume_empty_async_iterator():
    body_ran = False
    else_ran = False
    async for item in EmptyAsyncIterator():
        body_ran = True
    else:
        else_ran = True
    return (body_ran, else_ran)

empty_consumption = consume_empty_async_iterator()
try:
    empty_consumption.send(None)
except StopIteration as stopped:
    assert stopped.value == (False, True)
else:
    assert False
