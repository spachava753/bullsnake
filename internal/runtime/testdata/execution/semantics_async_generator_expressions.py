# Runtime execution cases for asynchronous generator expressions.
# case: async generator expressions are lazy and await elements
expression_activity = 0

async def expression_values():
    global expression_activity
    expression_activity += 1
    yield 1
    yield 2
    yield 3

async def double_expression_value(value):
    return value * 2

async def collect_async_generator_expression():
    item = expression_values()
    offset = 1
    stream = (
        await double_expression_value(item) + offset
        async for item in item
        if item > 1
    )
    assert expression_activity == 0
    total = 0
    async for value in stream:
        total = total * 10 + value
    return total

expression_collection = collect_async_generator_expression()
try:
    expression_collection.send(None)
except StopIteration as stopped:
    assert stopped.value == 57
else:
    assert False
assert expression_activity == 1

# ---
# case: async generator expressions mix synchronous and asynchronous clauses
async def nested_expression_values(left):
    yield left
    yield left + 10

async def collect_nested_generator_expression():
    stream = (
        (left, right)
        for left in (1, 2)
        async for right in nested_expression_values(left)
    )
    total = 0
    async for left, right in stream:
        total = total * 100 + left * 10 + right
    return total

nested_expression_collection = collect_nested_generator_expression()
try:
    nested_expression_collection.send(None)
except StopIteration as stopped:
    assert stopped.value == 11212232
else:
    assert False
