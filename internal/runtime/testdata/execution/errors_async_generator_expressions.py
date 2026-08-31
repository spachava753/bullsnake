# Runtime exception cases for asynchronous generator expressions.
# case: async generator expression targets do not leak
# error: NameError
# message: "name 'item' is not defined"
async def empty_expression_values():
    if False:
        yield 1

async def read_async_generator_expression_target():
    stream = (item async for item in empty_expression_values())
    async for ignored in stream:
        pass
    return item

read_async_generator_expression_target().send(None)

# ---
# case: async generator expression iterator failures propagate
# error: ValueError
# message: "async generator expression failed"
async def failing_expression_values():
    yield 1
    raise ValueError('async generator expression failed')

async def consume_failing_generator_expression():
    stream = (item async for item in failing_expression_values())
    async for item in stream:
        pass

consume_failing_generator_expression().send(None)
