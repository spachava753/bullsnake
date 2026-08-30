# Runtime execution cases for synchronous context managers.
# case: normal context manager entry and exit
class Manager:
    def __init__(self, result):
        self.result = result
        self.entered = False
        self.exited = False
        self.normal = False

    def __enter__(self):
        self.entered = True
        return self.result

    def __exit__(self, kind, value, traceback):
        self.exited = True
        self.normal = kind is None and value is None and traceback is None
        return False

manager = Manager(42)
manager.__enter__ = None
with manager as value:
    observed = value
    inside = manager.entered and not manager.exited
assert observed == 42
assert inside
assert manager.exited
assert manager.normal
# ---
# case: multiple context managers unwind in reverse order
class State:
    pass

class OrderedManager:
    def __init__(self, state, enter_digit, exit_digit):
        self.state = state
        self.enter_digit = enter_digit
        self.exit_digit = exit_digit

    def __enter__(self):
        self.state.value = self.state.value * 10 + self.enter_digit
        return self.enter_digit

    def __exit__(self, kind, value, traceback):
        self.state.value = self.state.value * 10 + self.exit_digit
        return False

state = State()
state.value = 0
with OrderedManager(state, 1, 5) as first, OrderedManager(state, 2, 4) as second:
    state.value = state.value * 10 + 3
assert first == 1
assert second == 2
assert state.value == 12345
# ---
# case: context manager suppresses body exception
class Swallow:
    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        self.kind = kind
        self.value = value
        self.traceback = traceback
        return True

manager = Swallow()
problem = ValueError('handled')
with manager as entered:
    raise problem
continued = True
assert entered is manager
assert continued
assert manager.kind is ValueError
assert manager.value is problem
assert manager.traceback is None
# ---
# case: context manager handles assignment failure
class AssignmentManager:
    def __enter__(self):
        return (1,)

    def __exit__(self, kind, value, traceback):
        self.kind = kind
        return True

manager = AssignmentManager()
body_ran = False
with manager as (left, right):
    body_ran = True
assert not body_ran
assert manager.kind is ValueError
# ---
# case: context exit replacement keeps exception context
class ReplacingManager:
    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        raise TypeError('replacement')

original = ValueError('original')
try:
    with ReplacingManager():
        raise original
except TypeError as replacement:
    context_preserved = replacement.__context__ is original
assert context_preserved
# ---
# case: context cleanup on return and loop transfer
class CounterManager:
    def __init__(self):
        self.exits = 0

    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        self.exits += 1
        return False

def return_from(manager):
    with manager:
        return 7

returned_manager = CounterManager()
returned = return_from(returned_manager)
break_manager = CounterManager()
for item in (1, 2):
    with break_manager:
        break
continue_manager = CounterManager()
visits = 0
for item in (1, 2, 3):
    with continue_manager:
        visits += 1
        continue
assert returned == 7
assert returned_manager.exits == 1
assert break_manager.exits == 1
assert continue_manager.exits == 3
assert visits == 3
# ---
# case: enter failure skips exit
class EnterFailure:
    def __init__(self):
        self.exited = False

    def __enter__(self):
        raise RuntimeError('enter failed')

    def __exit__(self, kind, value, traceback):
        self.exited = True

manager = EnterFailure()
try:
    with manager:
        body_ran = True
except RuntimeError:
    caught = True
assert caught
assert not manager.exited
# ---
# case: later enter failure unwinds earlier manager
class OuterManager:
    def __init__(self):
        self.exits = 0

    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        self.exits += 1
        self.kind = kind
        return False

class LaterFailure:
    def __init__(self):
        self.exited = False

    def __enter__(self):
        raise RuntimeError('later enter failed')

    def __exit__(self, kind, value, traceback):
        self.exited = True

outer = OuterManager()
later = LaterFailure()
try:
    with outer, later:
        body_ran = True
except RuntimeError:
    caught = True
assert caught
assert outer.exits == 1
assert outer.kind is RuntimeError
assert not later.exited
