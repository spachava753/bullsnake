# case: basic generic functions expose and capture type parameters
def reveal[T](value):
    return (value, T)

parameters = reveal.__type_params__
T = parameters[0]
assert T.__name__ == 'T'
assert f'{T!r}' == 'T'
result = reveal(7)
assert result[0] == 7
assert result[1] is T
assert reveal.__type_params__ is parameters

def pair[T, U](left, right):
    return (left, right, T, U)

pair_parameters = pair.__type_params__
pair_result = pair('left', 'right')
assert pair_result[0] == 'left'
assert pair_result[1] == 'right'
assert pair_result[2] is pair_parameters[0]
assert pair_result[3] is pair_parameters[1]
assert pair_parameters[0] is not pair_parameters[1]

def ordinary(value):
    return value

ordinary_parameters = ordinary.__type_params__
assert ordinary_parameters == ()
assert ordinary.__type_params__ is ordinary_parameters

# ---
# case: generic function annotations capture their type parameters lazily
annotation_calls = 0

def observe(value):
    global annotation_calls
    annotation_calls += 1
    return value

def identity[T](value: observe(T)) -> T:
    return value

identity_parameter = identity.__type_params__[0]
assert annotation_calls == 0
identity_annotations = identity.__annotations__
assert annotation_calls == 1
assert identity_annotations['value'] is identity_parameter
assert identity_annotations['return'] is identity_parameter
assert identity.__annotations__ is identity_annotations

# ---
# case: future generic function annotations retain source strings
from __future__ import annotations

def future_identity[T](value: T) -> T:
    return value

future_annotations = future_identity.__annotations__
assert future_annotations['value'] == 'T'
assert future_annotations['return'] == 'T'
assert future_identity.__type_params__[0].__name__ == 'T'

# ---
# case: generic function defaults are created in the defining scope
created = 0
default_value = []

def make_default():
    global created
    created += 1
    return default_value

def choose[T](value=make_default(), *, flag=True):
    return (value, flag, T)

assert created == 1
choose_parameter = choose.__type_params__[0]
first_choice = choose()
second_choice = choose()
assert first_choice[0] is default_value
assert second_choice[0] is default_value
assert first_choice[1] is True
assert first_choice[2] is choose_parameter
explicit_choice = choose('value', flag=False)
assert explicit_choice[0] == 'value'
assert explicit_choice[1] is False
assert explicit_choice[2] is choose_parameter
assert created == 1

# ---
# case: generic function decorators keep Python evaluation and application order
events = 0

def decorator_factory(digit):
    global events
    events = events * 10 + digit
    def apply(function):
        global events
        events = events * 10 + digit
        return function
    return apply

def decorated_default():
    global events
    events = events * 10 + 3
    return 7

@decorator_factory(1)
@decorator_factory(2)
def decorated[T](value=decorated_default()):
    return (value, T)

assert events == 12321
decorated_parameter = decorated.__type_params__[0]
decorated_result = decorated()
assert decorated_result[0] == 7
assert decorated_result[1] is decorated_parameter
