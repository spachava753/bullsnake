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
