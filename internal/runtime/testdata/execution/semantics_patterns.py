# Runtime execution cases for basic structural pattern matching.
# case: match literal and singleton patterns

def classify(value):
    match value:
        case None:
            return 'none'
        case True:
            return 'true'
        case False:
            return 'false'
        case 0:
            return 'zero'
        case -1:
            return 'negative'
        case 'word':
            return 'word'
        case 2j:
            return 'complex'
        case _:
            return 'other'

assert classify(None) == 'none'
assert classify(True) == 'true'
assert classify(False) == 'false'
assert classify(0) == 'zero'
assert classify(1) == 'other'
assert classify(-1) == 'negative'
assert classify('word') == 'word'
assert classify(2j) == 'complex'
# ---
# case: match dotted value evaluates subject once
subject_calls = 0

class Constants:
    pass

Constants.answer = 42

def matched_subject():
    global subject_calls
    subject_calls += 1
    return 42

match matched_subject():
    case Constants.answer:
        dotted_result = 'matched'
    case _:
        dotted_result = 'missed'

assert dotted_result == 'matched'
assert subject_calls == 1
# ---
# case: match OR AS and guard bindings

def guarded_choice(value, ready):
    match value:
        case 1 | 2 as number if ready:
            return number
        case captured:
            return captured + 10

assert guarded_choice(1, True) == 1
assert guarded_choice(2, True) == 2
assert guarded_choice(2, False) == 12
assert guarded_choice(5, True) == 15

guard_calls = 0

def reject_guard(value):
    global guard_calls
    guard_calls += value
    return False

match 4:
    case kept if reject_guard(kept):
        guard_result = 'accepted'
    case _:
        guard_result = 'rejected'

assert guard_result == 'rejected'
assert guard_calls == 4
assert kept == 4
# ---
# case: match commits captures only after pattern success
match 3:
    case 1 as missing_capture:
        rollback_result = 'wrong'
    case (2 as choice) | (3 as choice):
        rollback_result = choice

assert rollback_result == 3
try:
    missing_capture
except NameError:
    missing_capture_absent = True
assert missing_capture_absent

try:
    _
except NameError:
    wildcard_absent = True
assert wildcard_absent
# ---
# case: match capture can become a closure cell

def capture_reader(value):
    match value:
        case captured:
            def read():
                return captured
            return read

reader = capture_reader(7)
assert reader() == 7
