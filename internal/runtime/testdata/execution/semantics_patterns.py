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
# ---
# case: match fixed and nested sequence patterns

def sequence_kind(value):
    match value:
        case [first, [left, right]]:
            return first * 100 + left * 10 + right
        case [first, second]:
            return first * 10 + second
        case _:
            return -1

assert sequence_kind((1, 2)) == 12
assert sequence_kind([3, 4]) == 34
assert sequence_kind([5, (6, 7)]) == 567
assert sequence_kind([1]) == -1
assert sequence_kind([1, 2, 3]) == -1
assert sequence_kind('ab') == -1
assert sequence_kind(b'ab') == -1
# ---
# case: match starred sequence patterns

def split_sequence(value):
    match value:
        case [first, *middle, last]:
            return first, middle, last
        case [only]:
            return only, [], only
        case _:
            return None

first, middle, last = split_sequence((1, 2, 3, 4))
assert first == 1
middle_first, middle_second = middle
assert middle_first == 2
assert middle_second == 3
assert last == 4
single, empty, repeated = split_sequence([8])
assert single == 8
empty_count = 0
for empty_item in empty:
    empty_count += 1
assert empty_count == 0
assert repeated == 8
assert split_sequence([]) is None
# ---
# case: match sequence OR alternatives and rollback captures
match [2, 3]:
    case [choice] | [_, choice]:
        sequence_choice = choice

assert sequence_choice == 3

match [1]:
    case [missing_first, missing_second]:
        sequence_rollback = 'wrong'
    case _:
        sequence_rollback = 'clean'

assert sequence_rollback == 'clean'
try:
    missing_first
except NameError:
    first_capture_absent = True
try:
    missing_second
except NameError:
    second_capture_absent = True
assert first_capture_absent
assert second_capture_absent
# ---
# case: match sequence guard keeps captures and closure cells

def sequence_guard(value):
    match value:
        case [captured] if False:
            return None
        case _:
            def read():
                return captured
            return read

reader = sequence_guard([11])
assert reader() == 11
# ---
# case: match mapping keys and rest capture
record = {'name': 'Ada', 'age': 37, 'city': 'London'}
match record:
    case {'name': name, 'age': age, **rest}:
        mapping_result = name, age

matched_name, matched_age = mapping_result
assert matched_name == 'Ada'
assert matched_age == 37
assert rest['city'] == 'London'
assert 'name' not in rest
assert 'age' not in rest
rest['country'] = 'UK'
assert 'country' not in record
# ---
# case: match mapping nested and dotted keys

class MappingKeys:
    pass

MappingKeys.point = 'point'
match {'point': [4, 5], 'extra': 9}:
    case {MappingKeys.point: [x, y]}:
        nested_mapping_result = x * 10 + y

assert nested_mapping_result == 45

match {'right': 8}:
    case {'left': mapping_choice} | {'right': mapping_choice}:
        mapping_or_result = mapping_choice

assert mapping_or_result == 8
# ---
# case: match mapping failures do not bind captures
match {'name': 'Ada'}:
    case {'name': failed_name, 'age': failed_age}:
        mapping_rollback = 'wrong'
    case {}:
        mapping_rollback = 'clean'

assert mapping_rollback == 'clean'
try:
    failed_name
except NameError:
    failed_name_absent = True
try:
    failed_age
except NameError:
    failed_age_absent = True
assert failed_name_absent
assert failed_age_absent

match []:
    case {}:
        non_mapping_result = 'wrong'
    case _:
        non_mapping_result = 'clean'
assert non_mapping_result == 'clean'
# ---
# case: match mapping guard keeps committed captures
match {'value': 6, 'extra': 7}:
    case {'value': kept_value, **kept_rest} if False:
        mapping_guard_result = 'wrong'
    case _:
        mapping_guard_result = 'clean'

assert mapping_guard_result == 'clean'
assert kept_value == 6
assert kept_rest['extra'] == 7
# ---
# case: match class keyword and positional attributes

class Point:
    __match_args__ = ('x', 'y')

    def __init__(self, x, y):
        self.x = x
        self.y = y

point = Point(3, 4)
match point:
    case Point(x=keyword_x, y=keyword_y):
        keyword_result = keyword_x * 10 + keyword_y
assert keyword_result == 34

match point:
    case Point(positional_x, positional_y):
        positional_result = positional_x * 10 + positional_y
assert positional_result == 34
# ---
# case: match class accepts subclasses and nested patterns

class Point:
    __match_args__ = ('x', 'y')

    def __init__(self, x, y):
        self.x = x
        self.y = y

class ColoredPoint(Point):
    pass

class Box:
    def __init__(self, value):
        self.value = value

colored = ColoredPoint(5, 6)
boxed = Box(colored)
match boxed:
    case Box(value=Point(x, y)):
        nested_class_result = x * 10 + y
assert nested_class_result == 56

match colored:
    case Point(first, second):
        inherited_class_result = first + second
assert inherited_class_result == 11
# ---
# case: match class missing attributes and noninstances fail normally

class Point:
    pass

class Incomplete:
    pass

match Incomplete():
    case Incomplete(required=missing_attribute):
        missing_attribute_result = 'wrong'
    case _:
        missing_attribute_result = 'clean'
assert missing_attribute_result == 'clean'
try:
    missing_attribute
except NameError:
    class_capture_absent = True
assert class_capture_absent

match 7:
    case Point(x=not_a_point):
        noninstance_result = 'wrong'
    case _:
        noninstance_result = 'clean'
assert noninstance_result == 'clean'
# ---
# case: match class guard keeps captures

class Point:
    __match_args__ = ('x', 'y')

    def __init__(self, x, y):
        self.x = x
        self.y = y

point = Point(3, 4)
match point:
    case Point(guarded_x, guarded_y) as guarded_point if False:
        class_guard_result = 'wrong'
    case _:
        class_guard_result = 'clean'
assert class_guard_result == 'clean'
assert guarded_x == 3
assert guarded_y == 4
assert guarded_point is point
# ---
# case: match class supports exception classes
error = ValueError('bad')
match error:
    case ValueError():
        builtin_exception_result = True
assert builtin_exception_result

class Problem(ValueError):
    pass

problem = Problem('custom')
match problem:
    case Problem():
        user_exception_result = True
assert user_exception_result

match problem:
    case ValueError():
        exception_parent_result = True
assert exception_parent_result
