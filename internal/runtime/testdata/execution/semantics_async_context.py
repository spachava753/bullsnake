# Runtime execution cases for asynchronous context managers.
# case: async context manager binds and exits normally
class AsyncManager:
    def __init__(self, result):
        self.result = result
        self.entered = False
        self.exited = False
        self.normal = False

    async def __aenter__(self):
        self.entered = True
        return self.result

    async def __aexit__(self, kind, value, traceback):
        self.exited = True
        self.normal = kind is None and value is None and traceback is None
        return False

async def use_async_manager(manager):
    async with manager as value:
        assert manager.entered
        assert not manager.exited
        return value

manager = AsyncManager(42)
computation = use_async_manager(manager)
try:
    computation.send(None)
except StopIteration as stopped:
    assert stopped.value == 42
else:
    assert False
assert manager.exited
assert manager.normal

# ---
# case: multiple async context managers unwind in reverse order
class AsyncOrderState:
    pass

class OrderedAsyncManager:
    def __init__(self, state, enter_digit, exit_digit):
        self.state = state
        self.enter_digit = enter_digit
        self.exit_digit = exit_digit

    async def __aenter__(self):
        self.state.value = self.state.value * 10 + self.enter_digit
        return self.enter_digit

    async def __aexit__(self, kind, value, traceback):
        self.state.value = self.state.value * 10 + self.exit_digit
        return False

async def use_ordered_managers(state):
    async with OrderedAsyncManager(state, 1, 5) as first, OrderedAsyncManager(state, 2, 4) as second:
        state.value = state.value * 10 + 3
        return (first, second)

state = AsyncOrderState()
state.value = 0
ordered = use_ordered_managers(state)
try:
    ordered.send(None)
except StopIteration as stopped:
    assert stopped.value == (1, 2)
else:
    assert False
assert state.value == 12345

# ---
# case: async context manager suppresses a body exception
class AsyncSuppressor:
    async def __aenter__(self):
        return self

    async def __aexit__(self, kind, value, traceback):
        self.kind = kind
        self.value = value
        self.traceback = traceback
        return True

async def use_suppressor(manager, problem):
    async with manager as entered:
        assert entered is manager
        raise problem
    return 'continued'

suppressor = AsyncSuppressor()
problem = ValueError('handled asynchronously')
suppressed = use_suppressor(suppressor, problem)
try:
    suppressed.send(None)
except StopIteration as stopped:
    assert stopped.value == 'continued'
else:
    assert False
assert suppressor.kind is ValueError
assert suppressor.value is problem
assert suppressor.traceback is None

# ---
# case: async context cleanup runs during return and loop transfer
class AsyncCounterManager:
    def __init__(self):
        self.exits = 0

    async def __aenter__(self):
        return self

    async def __aexit__(self, kind, value, traceback):
        self.exits += 1
        return False

async def return_from_async_context(manager):
    async with manager:
        return 7

async def leave_async_context_loops(break_manager, continue_manager):
    for item in (1, 2):
        async with break_manager:
            break
    visits = 0
    for item in (1, 2, 3):
        async with continue_manager:
            visits += 1
            continue
    return visits

returned_manager = AsyncCounterManager()
break_manager = AsyncCounterManager()
continue_manager = AsyncCounterManager()
returning = return_from_async_context(returned_manager)
try:
    returning.send(None)
except StopIteration as stopped:
    assert stopped.value == 7
else:
    assert False
leaving = leave_async_context_loops(break_manager, continue_manager)
try:
    leaving.send(None)
except StopIteration as stopped:
    assert stopped.value == 3
else:
    assert False
assert returned_manager.exits == 1
assert break_manager.exits == 1
assert continue_manager.exits == 3

# ---
# case: later async entry failure unwinds an earlier manager
class OuterAsyncManager:
    def __init__(self):
        self.exits = 0

    async def __aenter__(self):
        return self

    async def __aexit__(self, kind, value, traceback):
        self.exits += 1
        self.kind = kind
        return False

class FailingAsyncManager:
    def __init__(self):
        self.exited = False

    async def __aenter__(self):
        raise RuntimeError('later async enter failed')

    async def __aexit__(self, kind, value, traceback):
        self.exited = True

async def enter_two_async_managers(outer, later):
    try:
        async with outer, later:
            return False
    except RuntimeError:
        return True

outer = OuterAsyncManager()
later = FailingAsyncManager()
entering = enter_two_async_managers(outer, later)
try:
    entering.send(None)
except StopIteration as stopped:
    assert stopped.value is True
else:
    assert False
assert outer.exits == 1
assert outer.kind is RuntimeError
assert not later.exited
