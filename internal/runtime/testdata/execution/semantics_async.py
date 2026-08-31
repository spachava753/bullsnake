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
