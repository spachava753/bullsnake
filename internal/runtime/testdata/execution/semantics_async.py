# case: coroutine calls are lazy and return through StopIteration
coroutine_started = False

async def compute(value):
    global coroutine_started
    coroutine_started = True
    return value + 1

computation = compute(41)
assert coroutine_started is False
assert f'{computation!r}' == '<coroutine object compute>'
try:
    computation.send(None)
except StopIteration as stopped:
    assert stopped.value == 42
else:
    assert False
assert coroutine_started is True

# ---
# case: coroutine frames retain defaults and closures

def make_coroutine(offset):
    async def add(value=2):
        return offset + value
    return add

adder = make_coroutine(40)
default_computation = adder()
try:
    default_computation.send(None)
except StopIteration as stopped:
    assert stopped.value == 42
else:
    assert False

explicit_computation = adder(3)
try:
    explicit_computation.send(None)
except StopIteration as stopped:
    assert stopped.value == 43
else:
    assert False

# ---
# case: await drives nested native coroutines
await_steps = 0

async def awaited_inner(value):
    global await_steps
    await_steps = await_steps * 10 + 1
    return value + 1

async def awaited_outer(value):
    global await_steps
    await_steps = await_steps * 10 + 2
    result = await awaited_inner(value)
    await_steps = await_steps * 10 + 3
    return result * 2

awaited_computation = awaited_outer(41)
assert await_steps == 0
try:
    awaited_computation.send(None)
except StopIteration as stopped:
    assert stopped.value == 84
else:
    assert False
assert await_steps == 213

# ---
# case: await propagates exceptions through handlers and finally
await_finally_ran = False

async def awaited_failure():
    raise ValueError('awaited failure')

async def recover_awaited_failure():
    global await_finally_ran
    try:
        try:
            await awaited_failure()
        except ValueError as error:
            assert f'{error}' == 'awaited failure'
            return 'recovered'
    finally:
        await_finally_ran = True

recovery = recover_awaited_failure()
try:
    recovery.send(None)
except StopIteration as stopped:
    assert stopped.value == 'recovered'
else:
    assert False
assert await_finally_ran is True
