# Runtime execution cases for calls.
# case: default argument binding
seed = 10
def choose(first=seed, second=seed + 1):
    return first, second
seed = 99
both_defaulted = choose()
second_defaulted = choose(20)
none_defaulted = choose(20, 30)
def combine(required, optional=5):
    return required + optional
combined = combine(7)
def positional(first=1, /, second=2):
    return first, second
positional_defaults = positional()
marker = []
def retain(value=marker):
    return value
default_identity = retain() is marker
assert f'{both_defaulted!r}' == "(10, 11)", "both_defaulted"
assert f'{second_defaulted!r}' == "(20, 11)", "second_defaulted"
assert f'{none_defaulted!r}' == "(20, 30)", "none_defaulted"
assert f'{combined!r}' == "12", "combined"
assert f'{positional_defaults!r}' == "(1, 2)", "positional_defaults"
assert f'{default_identity!r}' == "True", "default_identity"
# ---
# case: variadic argument binding
def collect(first, *items):
    local = first
    return local, items
empty_items = collect(1)
many_items = collect(1, 2, 3, 4)
def only(*items):
    return items
only_empty = only()
only_many = only(5, 6)
def defaulted(first=7, *items):
    return first, items
default_empty = defaulted()
default_many = defaulted(8, 9, 10)
assert f'{empty_items!r}' == "(1, ())", "empty_items"
assert f'{many_items!r}' == "(1, (2, 3, 4))", "many_items"
assert f'{only_empty!r}' == "()", "only_empty"
assert f'{only_many!r}' == "(5, 6)", "only_many"
assert f'{default_empty!r}' == "(7, ())", "default_empty"
assert f'{default_many!r}' == "(8, (9, 10))", "default_many"
# ---
# case: call argument expansion
def add(left, right):
    return left + right
from_tuple = add(*(40, 2))
from_list = add(*[20, 22])
def collect(*items):
    return items
mixed = collect(1, *[2, 3], 4, *(5, 6))
empty = collect(*())
assert f'{from_tuple!r}' == "42", "from_tuple"
assert f'{from_list!r}' == "42", "from_list"
assert f'{mixed!r}' == "(1, 2, 3, 4, 5, 6)", "mixed"
assert f'{empty!r}' == "()", "empty"
# ---
# case: keyword argument calls
def combine(first, second, third=3):
    return first, second, third
all_keywords = combine(first=1, second=2)
mixed = combine(4, third=6, second=5)
options = {'second': 8}
unpacked = combine(7, **options)
expanded = combine(*(9,), **{'second': 10, 'third': 11})
def positional(first, /, second):
    return first, second
positional_ok = positional(12, second=13)
assert f'{all_keywords!r}' == "(1, 2, 3)", "all_keywords"
assert f'{mixed!r}' == "(4, 5, 6)", "mixed"
assert f'{unpacked!r}' == "(7, 8, 3)", "unpacked"
assert f'{expanded!r}' == "(9, 10, 11)", "expanded"
assert f'{positional_ok!r}' == "(12, 13)", "positional_ok"
# ---
# case: named only parameters
def configure(*, required, mode='safe', retries=3, verbose):
    return required, mode, retries, verbose
defaulted = configure(required=1, verbose=True)
overridden = configure(required=2, mode='fast', retries=5, verbose=False)
def mixed(first=10, *items, flag, mode='mixed'):
    return first, items, flag, mode
mixed_default = mixed(flag=True)
mixed_values = mixed(20, 30, 40, flag=False, mode='custom')
marker = []
def retain(*, value=marker):
    return value
default_identity = retain() is marker
assert f'{defaulted!r}' == "(1, 'safe', 3, True)", "defaulted"
assert f'{overridden!r}' == "(2, 'fast', 5, False)", "overridden"
assert f'{mixed_default!r}' == "(10, (), True, 'mixed')", "mixed_default"
assert f'{mixed_values!r}' == "(20, (30, 40), False, 'custom')", "mixed_values"
assert f'{default_identity!r}' == "True", "default_identity"
# ---
# case: catch all keyword binding
def collect(first=1, **options):
    return first, options
empty = collect()
filled = collect(2, mode='fast', retries=3)
def mixed(first, *items, flag='default', **options):
    return first, items, flag, options
combined = mixed(10, 20, flag='set', extra=30)
def preserve(name, /, **options):
    return name, options
preserved = preserve('bound', name='extra')
def fresh(**options):
    return options
distinct = fresh() is not fresh()
assert f'{empty!r}' == "(1, {})", "empty"
assert f'{filled!r}' == "(2, {'mode': 'fast', 'retries': 3})", "filled"
assert f'{combined!r}' == "(10, (20,), 'set', {'extra': 30})", "combined"
assert f'{preserved!r}' == "('bound', {'name': 'extra'})", "preserved"
assert f'{distinct!r}' == "True", "distinct"
# ---
# case: positional extrema builtins
assert max(1.0, 3.0, 2.0) == 3.0
assert min(1.0, -2.0, 0.0) == -2.0
assert max('alpha', 'gamma', 'beta') == 'gamma'
assert min(b'alpha', b'gamma', b'beta') == b'alpha'
# ---
# case: scalar int builtin
assert int(4.9) == 4
assert int(-4.9) == -4
assert int(7) == 7
assert int(True) == 1
