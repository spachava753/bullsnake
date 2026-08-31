# Runtime execution cases for asynchronous comprehensions.
# case: async comprehensions build list set and dictionary values
class AsyncComprehensionValues:
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

async def double_async_value(value):
    return value * 2

async def keep_async_value(value):
    return value > 1

async def build_async_comprehensions():
    values = [
        await double_async_value(item)
        async for item in AsyncComprehensionValues((1, 2, 3), 3)
        if await keep_async_value(item)
    ]
    members = {
        item
        async for item in AsyncComprehensionValues((1, 2, 2), 3)
    }
    mapping = {
        item: item * 10
        async for item in AsyncComprehensionValues((1, 2), 2)
    }
    return (values, members, mapping)

async def read_outer_first_iterable():
    item = AsyncComprehensionValues((9,), 1)
    return [item async for item in item]

building = build_async_comprehensions()
try:
    building.send(None)
except StopIteration as stopped:
    values, members, mapping = stopped.value
else:
    assert False
first, second = values
assert first == 4
assert second == 6
assert 1 in members
assert 2 in members
assert mapping[1] == 10
assert mapping[2] == 20

outer_reading = read_outer_first_iterable()
try:
    outer_reading.send(None)
except StopIteration as stopped:
    only, = stopped.value
else:
    assert False
assert only == 9

# ---
# case: async comprehensions mix and nest clause kinds
class NestedAsyncValues:
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

async def build_nested_async_comprehensions():
    mixed = [
        (left, right)
        for left in (1, 2)
        async for right in NestedAsyncValues((left, left + 10), 2)
    ]
    nested = [
        (left, right)
        async for left in NestedAsyncValues((1, 2), 2)
        async for right in NestedAsyncValues((left,), 1)
    ]
    return (mixed, nested)

nested_building = build_nested_async_comprehensions()
try:
    nested_building.send(None)
except StopIteration as stopped:
    mixed, nested = stopped.value
else:
    assert False
mixed_first, mixed_second, mixed_third, mixed_fourth = mixed
assert mixed_first == (1, 1)
assert mixed_second == (1, 11)
assert mixed_third == (2, 2)
assert mixed_fourth == (2, 12)
nested_first, nested_second = nested
assert nested_first == (1, 1)
assert nested_second == (2, 2)
