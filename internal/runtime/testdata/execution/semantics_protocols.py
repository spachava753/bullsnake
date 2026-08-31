# Runtime execution cases for user-defined object protocols.
# case: user truth protocols
class Decision:
    def __init__(self, value):
        self.value = value
        self.calls = 0
    def __bool__(self):
        self.calls = self.calls + 1
        return self.value

accepted = Decision(True)
rejected = Decision(False)
if accepted:
    branch = 'accepted'
else:
    branch = missing
if rejected:
    missing
else:
    other_branch = 'rejected'
negated = not rejected
selected_and = accepted and 'right'
selected_or = rejected or 'fallback'
conditional = 10 if accepted else missing
assert accepted

class Sized:
    def __init__(self, size):
        self.size = size
    def __len__(self):
        return self.size

empty = Sized(0)
nonempty = Sized(3)
empty_result = not empty
if nonempty:
    sized_branch = True
else:
    sized_branch = missing

class Both:
    def __bool__(self):
        return False
    def __len__(self):
        return 4

precedence = not Both()

class BaseTruth:
    def __bool__(self):
        return False

class ChildTruth(BaseTruth):
    pass

inherited = not ChildTruth()

class Ordinary:
    pass

def false_function():
    return False

ordinary = Ordinary()
ordinary.__bool__ = false_function
instance_attribute_ignored = ordinary and True

assert branch == 'accepted'
assert other_branch == 'rejected'
assert negated is True
assert selected_and == 'right'
assert selected_or == 'fallback'
assert conditional == 10
assert accepted.calls == 4
assert rejected.calls == 3
assert empty_result is True
assert sized_branch is True
assert precedence is True
assert inherited is True
assert instance_attribute_ignored is True
# ---
# case: truth method exceptions are catchable
class BrokenTruth:
    def __bool__(self):
        raise ValueError('truth failed')

try:
    if BrokenTruth():
        missing
except ValueError as error:
    caught = f'{error!r}'

assert caught == 'ValueError("truth failed")'
