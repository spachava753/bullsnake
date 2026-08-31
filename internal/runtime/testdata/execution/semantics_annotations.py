# case: class annotations are lazy and preserve assignments
events = 0

def mark(value):
    global events
    events = events + 1
    return value

class Model:
    Kind = 'class kind'
    stored: mark(Kind) = 7
    declared: mark('declared kind')

before = events
stored_value = Model.stored
annotations = Model.__annotate__(1)
after = events
assert before == 0
assert stored_value == 7
assert annotations['stored'] == 'class kind'
assert annotations['declared'] == 'declared kind'
assert after == 2
# ---
# case: class annotations record only executed statements
ready = False

class Conditional:
    if ready:
        skipped: 'skip'
    else:
        kept: 'keep'

conditional_annotations = Conditional.__annotate__(1)
assert conditional_annotations['kept'] == 'keep'
try:
    conditional_annotations['skipped']
except KeyError:
    skipped_absent = True
assert skipped_absent
# ---
# case: class annotations see class enclosing and global names
GlobalKind = 'global kind'

def make_model(OuterKind):
    class NestedModel:
        LocalKind = 'local kind'
        local_value: LocalKind
        outer_value: OuterKind
        global_value: GlobalKind
    return NestedModel

NestedModel = make_model('outer kind')
nested_annotations = NestedModel.__annotate__(1)
assert nested_annotations['local_value'] == 'local kind'
assert nested_annotations['outer_value'] == 'outer kind'
assert nested_annotations['global_value'] == 'global kind'
# ---
# case: explicit class annotate wins and is not inherited
class Base:
    value: int

    def __annotate__(format):
        return {'manual': format}

class Child(Base):
    pass

manual_annotations = Base.__annotate__(1)
assert manual_annotations['manual'] == 1
assert Child.__annotate__ is None
# ---
# case: class annotations evaluate once and cache identity
events = 0

def mark():
    global events
    events = events + 1
    return 'kind'

class Cached:
    value: mark()

before = events
first_annotations = Cached.__annotations__
after_first = events
second_annotations = Cached.__annotations__
after_second = events
assert before == 0
assert after_first == 1
assert after_second == 1
assert first_annotations is second_annotations
assert first_annotations['value'] == 'kind'
# ---
# case: empty and explicit class annotation dictionaries
class Empty:
    pass

empty_first = Empty.__annotations__
empty_second = Empty.__annotations__
assert empty_first is empty_second
try:
    empty_first['missing']
except KeyError:
    empty_annotations = True
assert empty_annotations

manual = {'value': 42}
class ExplicitAnnotations:
    __annotations__ = manual

assert ExplicitAnnotations.__annotations__ is manual
# ---
# case: failed class annotation evaluation is retried
events = 0

def fail_annotation():
    global events
    events = events + 1
    raise ValueError('retry')

class Retry:
    value: fail_annotation()

try:
    Retry.__annotations__
except ValueError:
    first_failure = True
try:
    Retry.__annotations__
except ValueError:
    second_failure = True
assert first_failure
assert second_failure
assert events == 2
# ---
# case: future module annotations store source strings
from __future__ import annotations
ready = False
stored: Missing = 7
if ready:
    skipped: list[int]
else:
    kept: list [ int | str ]

assert stored == 7
assert __annotations__['stored'] == 'Missing'
assert __annotations__['kept'] == 'list [ int | str ]'
try:
    __annotations__['skipped']
except KeyError:
    skipped_absent = True
assert skipped_absent
# ---
# case: future annotation-only complex targets do nothing
from __future__ import annotations
calls = 0

class Holder:
    pass

holder = Holder()

def target():
    global calls
    calls = calls + 1
    return holder

target().field: Missing
assert calls == 0
# ---
# case: future class annotations store source strings
from __future__ import annotations
events = 0
ready = False

def mark():
    global events
    events = events + 1
    return int

class FutureModel:
    stored: tuple [ int, str ] = 7
    observed: mark()
    if ready:
        skipped: Missing
    else:
        kept: dict[str, list[int]]

assert FutureModel.stored == 7
assert events == 0
class_annotations = FutureModel.__annotations__
assert class_annotations['stored'] == 'tuple [ int, str ]'
assert class_annotations['observed'] == 'mark()'
assert class_annotations['kept'] == 'dict[str, list[int]]'
try:
    class_annotations['skipped']
except KeyError:
    class_skipped_absent = True
assert class_skipped_absent
