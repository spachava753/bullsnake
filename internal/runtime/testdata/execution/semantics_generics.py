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

# ---
# case: generic functions bind variadic positional and keyword arguments
def collect[T](head: T, *items: T, flag: T = True, **options: T) -> T:
    return (head, items, flag, options, T)

collect_parameter = collect.__type_params__[0]
collected = collect(1, 2, 3, flag=False, extra=4)
assert collected[0] == 1
assert collected[1] == (2, 3)
assert collected[2] is False
assert collected[3]['extra'] == 4
assert collected[4] is collect_parameter
collect_annotations = collect.__annotations__
assert collect_annotations['head'] is collect_parameter
assert collect_annotations['items'] is collect_parameter
assert collect_annotations['flag'] is collect_parameter
assert collect_annotations['options'] is collect_parameter
assert collect_annotations['return'] is collect_parameter

# ---
# case: generic function TypeVar bounds and constraints evaluate lazily
bound_calls = 0

class Bound:
    pass

class First:
    pass

class Second:
    pass

def resolve_bound():
    global bound_calls
    bound_calls += 1
    return Bound

def bounded[T: resolve_bound()]():
    return T

def constrained[T: (First, Second)]():
    return T

assert bound_calls == 0
bounded_parameter = bounded.__type_params__[0]
assert bounded_parameter.__bound__ is Bound
assert bound_calls == 1
assert bounded_parameter.__bound__ is Bound
assert bound_calls == 1
constraints = constrained.__type_params__[0].__constraints__
assert constraints[0] is First
assert constraints[1] is Second

# ---
# case: generic function TypeVar defaults evaluate lazily
function_default_calls = 0

class FunctionFallback:
    pass

def resolve_function_default():
    global function_default_calls
    function_default_calls += 1
    return FunctionFallback

def defaulted[T = resolve_function_default()]():
    return T

def dependent[T, U = T]():
    return (T, U)

def no_default[T]():
    return T

assert function_default_calls == 0
defaulted_parameter = defaulted.__type_params__[0]
assert defaulted_parameter.__default__ is FunctionFallback
assert function_default_calls == 1
assert defaulted_parameter.__default__ is FunctionFallback
assert function_default_calls == 1
dependent_parameters = dependent.__type_params__
assert dependent_parameters[1].__default__ is dependent_parameters[0]
assert f'{no_default.__type_params__[0].__default__!r}' == 'typing.NoDefault'

# ---
# case: generic functions create variadic type parameters and defaults
class VariadicFallback:
    pass

def variadic[*Ts, **P]():
    return (Ts, P)

def variadic_defaults[*Ts = (VariadicFallback,), **P = (VariadicFallback,)]():
    return (Ts, P)

variadic_parameters = variadic.__type_params__
assert f'{variadic_parameters[0]!r}' == 'Ts'
assert f'{variadic_parameters[1]!r}' == 'P'
variadic_result = variadic()
assert variadic_result[0] is variadic_parameters[0]
assert variadic_result[1] is variadic_parameters[1]
assert variadic_parameters[1].args is not variadic_parameters[1].kwargs
assert f'{variadic_parameters[0].__default__!r}' == 'typing.NoDefault'
assert f'{variadic_parameters[1].__default__!r}' == 'typing.NoDefault'
default_parameters = variadic_defaults.__type_params__
assert default_parameters[0].__default__[0] is VariadicFallback
assert default_parameters[1].__default__[0] is VariadicFallback
