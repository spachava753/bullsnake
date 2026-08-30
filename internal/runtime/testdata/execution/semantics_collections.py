# Runtime execution cases for collections.
# case: collection displays
empty_tuple = ()
tuple_value = (1, True, 'text')
single_tuple = (1,)
empty_list = []
list_value = [1, None, [2, 3]]
tuple_is_false = not empty_tuple
list_is_false = not empty_list
list_is_true = not list_value
assert f'{empty_tuple!r}' == "()", "empty_tuple"
assert f'{tuple_value!r}' == "(1, True, 'text')", "tuple_value"
assert f'{single_tuple!r}' == "(1,)", "single_tuple"
assert f'{empty_list!r}' == "[]", "empty_list"
assert f'{list_value!r}' == "[1, None, [2, 3]]", "list_value"
assert f'{tuple_is_false!r}' == "True", "tuple_is_false"
assert f'{list_is_false!r}' == "True", "list_is_false"
assert f'{list_is_true!r}' == "False", "list_is_true"
# ---
# case: sequence indexing
tuple_value = (10, 20, 30)
tuple_first = tuple_value[0]
tuple_last = tuple_value[-1]
tuple_bool = tuple_value[True]
list_value = [40, 50, 60]
list_first = list_value[0]
list_last = list_value[-1]
list_bool = list_value[False]
nested = [(1, 2), [3, 4]][1][0]
assert f'{tuple_first!r}' == "10", "tuple_first"
assert f'{tuple_last!r}' == "30", "tuple_last"
assert f'{tuple_bool!r}' == "20", "tuple_bool"
assert f'{list_first!r}' == "40", "list_first"
assert f'{list_last!r}' == "60", "list_last"
assert f'{list_bool!r}' == "40", "list_bool"
assert f'{nested!r}' == "3", "nested"
# ---
# case: text and bytes subscription
text = 'A\u00e9\u2603\ud800Z'
text_first = text[0]
text_accent = text[1]
text_snowman = text[2]
text_surrogate = text[3]
text_last = text[-1]
text_bool = text[True]
text_middle = text[1:4]
text_reverse = text[::-1]
text_stride = text[4:0:-2]
text_empty = text[10:]
text_same = text[:] is text
text_snake = '\U0001f40d'[0]
data = b'\x00A\xff'
byte_first = data[0]
byte_middle = data[1]
byte_last = data[-1]
byte_bool = data[True]
bytes_tail = data[1:]
bytes_reverse = data[::-1]
bytes_stride = data[::2]
bytes_empty = data[3:1]
bytes_same = data[:] is data
assert f'{text_first!r}' == "'A'", "text_first"
assert f'{text_accent!r}' == "'é'", "text_accent"
assert f'{text_snowman!r}' == "'☃'", "text_snowman"
assert f'{text_surrogate!r}' == "'\\ud800'", "text_surrogate"
assert f'{text_last!r}' == "'Z'", "text_last"
assert f'{text_bool!r}' == "'é'", "text_bool"
assert f'{text_middle!r}' == "'é☃\\ud800'", "text_middle"
assert f'{text_reverse!r}' == "'Z\\ud800☃éA'", "text_reverse"
assert f'{text_stride!r}' == "'Z☃'", "text_stride"
assert f'{text_empty!r}' == "''", "text_empty"
assert f'{text_same!r}' == "True", "text_same"
assert f'{text_snake!r}' == "'🐍'", "text_snake"
assert f'{byte_first!r}' == "0", "byte_first"
assert f'{byte_middle!r}' == "65", "byte_middle"
assert f'{byte_last!r}' == "255", "byte_last"
assert f'{byte_bool!r}' == "65", "byte_bool"
assert f'{bytes_tail!r}' == "b'A\\xff'", "bytes_tail"
assert f'{bytes_reverse!r}' == "b'\\xffA\\x00'", "bytes_reverse"
assert f'{bytes_stride!r}' == "b'\\x00\\xff'", "bytes_stride"
assert f'{bytes_empty!r}' == "b''", "bytes_empty"
assert f'{bytes_same!r}' == "True", "bytes_same"
# ---
# case: tuple and list slicing
tuple_value = (0, 1, 2, 3, 4)
tuple_middle = tuple_value[1:4]
tuple_reverse = tuple_value[::-1]
tuple_stride = tuple_value[4:0:-2]
tuple_clipped = tuple_value[-100:100]
tuple_same = tuple_value[:] is tuple_value
list_value = [0, 1, 2, 3, 4]
list_middle = list_value[-4:-1]
list_reverse = list_value[::-1]
list_stride = list_value[::2]
list_empty = list_value[3:1]
list_bool_bounds = list_value[False:True]
list_none_step = list_value[1:4:None]
list_huge = list_value[-1000000000000000000000000000000:1000000000000000000000000000000:1000000000000000000000000000000]
list_copy = list_value[:]
list_same = list_copy is list_value
assert f'{tuple_middle!r}' == "(1, 2, 3)", "tuple_middle"
assert f'{tuple_reverse!r}' == "(4, 3, 2, 1, 0)", "tuple_reverse"
assert f'{tuple_stride!r}' == "(4, 2)", "tuple_stride"
assert f'{tuple_clipped!r}' == "(0, 1, 2, 3, 4)", "tuple_clipped"
assert f'{tuple_same!r}' == "True", "tuple_same"
assert f'{list_middle!r}' == "[1, 2, 3]", "list_middle"
assert f'{list_reverse!r}' == "[4, 3, 2, 1, 0]", "list_reverse"
assert f'{list_stride!r}' == "[0, 2, 4]", "list_stride"
assert f'{list_empty!r}' == "[]", "list_empty"
assert f'{list_bool_bounds!r}' == "[0]", "list_bool_bounds"
assert f'{list_none_step!r}' == "[1, 2, 3]", "list_none_step"
assert f'{list_huge!r}' == "[0]", "list_huge"
assert f'{list_copy!r}' == "[0, 1, 2, 3, 4]", "list_copy"
assert f'{list_same!r}' == "False", "list_same"
# ---
# case: starred tuple and list displays
source = [1, 2]
list_value = [0, *source, 3, *(4, 5)]
tuple_value = (0, *source, 3, *[4, 5])
nested = [*[(1, 2)], *[[3, 4]]]
empty_list = [*()]
empty_tuple = (*[],)
assert f'{source!r}' == "[1, 2]", "source"
assert f'{list_value!r}' == "[0, 1, 2, 3, 4, 5]", "list_value"
assert f'{tuple_value!r}' == "(0, 1, 2, 3, 4, 5)", "tuple_value"
assert f'{nested!r}' == "[(1, 2), [3, 4]]", "nested"
assert f'{empty_list!r}' == "[]", "empty_list"
assert f'{empty_tuple!r}' == "()", "empty_tuple"
# ---
# case: dictionary displays
empty = {}
ordered = {'first': 1, 'second': 2}
duplicate = {'key': 1, 'other': 0, 'key': 2}
numeric = {True: 'bool', 1: 'int', 1.0: 'float'}
tuple_key = {(1, 2): 'pair'}
tuple_duplicate = {(1, 2): 'first', (1, 2): 'second'}
nested = {'list': [1, 2], 'dict': {'x': 3}}
empty_false = not empty
ordered_false = not ordered
assert f'{empty!r}' == "{}", "empty"
assert f'{ordered!r}' == "{'first': 1, 'second': 2}", "ordered"
assert f'{duplicate!r}' == "{'key': 2, 'other': 0}", "duplicate"
assert f'{numeric!r}' == "{True: 'float'}", "numeric"
assert f'{tuple_key!r}' == "{(1, 2): 'pair'}", "tuple_key"
assert f'{tuple_duplicate!r}' == "{(1, 2): 'second'}", "tuple_duplicate"
assert f'{nested!r}' == "{'list': [1, 2], 'dict': {'x': 3}}", "nested"
assert f'{empty_false!r}' == "True", "empty_false"
assert f'{ordered_false!r}' == "False", "ordered_false"
# ---
# case: mapping subscription
mapping = {'name': 'bullsnake', (1, 2): 'pair', True: 'truth'}
by_name = mapping['name']
by_tuple = mapping[(1, 2)]
by_integer = mapping[1]
nested = {'inner': {'value': 7}}['inner']['value']
assert f'{by_name!r}' == "'bullsnake'", "by_name"
assert f'{by_tuple!r}' == "'pair'", "by_tuple"
assert f'{by_integer!r}' == "'truth'", "by_integer"
assert f'{nested!r}' == "7", "nested"
# ---
# case: unpacked dictionary displays
base = {'first': 1, 'second': 2}
copied = {**base}
mixed = {'first': 0, **base, 'third': 3}
replaced = {**{'a': 1, 'b': 2}, **{'b': 20, 'c': 3}, 'a': 10}
empty = {**{}}
assert f'{base!r}' == "{'first': 1, 'second': 2}", "base"
assert f'{copied!r}' == "{'first': 1, 'second': 2}", "copied"
assert f'{mixed!r}' == "{'first': 1, 'second': 2, 'third': 3}", "mixed"
assert f'{replaced!r}' == "{'a': 10, 'b': 20, 'c': 3}", "replaced"
assert f'{empty!r}' == "{}", "empty"
# ---
# case: set displays
values = {3, 1, 2, 1}
numeric = {True, 1, 1.0, False, 0}
tuples = {(1, 2), (1, 2), (3, 4)}
starred = {0, *[1, 2], *(2, 3)}
from_set = {*{1, 2}, 3}
empty = {*()}
empty_false = not empty
values_false = not values
assert f'{values!r}' == "{3, 1, 2}", "values"
assert f'{numeric!r}' == "{True, False}", "numeric"
assert f'{tuples!r}' == "{(1, 2), (3, 4)}", "tuples"
assert f'{starred!r}' == "{0, 1, 2, 3}", "starred"
assert f'{from_set!r}' == "{1, 2, 3}", "from_set"
assert f'{empty!r}' == "set()", "empty"
assert f'{empty_false!r}' == "True", "empty_false"
assert f'{values_false!r}' == "False", "values_false"
# ---
# case: membership operations
tuple_hit = 2 in (1, 2, 3)
tuple_miss = 4 in (1, 2, 3)
list_not_in = 4 not in [1, 2, 3]
dict_key = 'key' in {'key': 1}
dict_value = 1 in {'key': 1}
dict_tuple = (1, 2) in {(1, 2): 'pair'}
set_numeric = 1 in {True}
set_miss = 3 in {1, 2}
set_not_in = 3 not in {1, 2}
text_character = '\u00e9' in 'caf\u00e9'
text_substring = 'af\u00e9' in 'caf\u00e9'
text_surrogate = '\ud800' in 'a\ud800b'
text_empty = '' in 'bullsnake'
text_not_in = 'python' not in 'bullsnake'
bytes_integer = 65 in b'\x00A\xff'
bytes_bool = False in b'\x00A\xff'
bytes_subsequence = b'A\xff' in b'\x00A\xff'
bytes_empty = b'' in b'bullsnake'
bytes_not_in = b'python' not in b'bullsnake'
assert f'{tuple_hit!r}' == "True", "tuple_hit"
assert f'{tuple_miss!r}' == "False", "tuple_miss"
assert f'{list_not_in!r}' == "True", "list_not_in"
assert f'{dict_key!r}' == "True", "dict_key"
assert f'{dict_value!r}' == "False", "dict_value"
assert f'{dict_tuple!r}' == "True", "dict_tuple"
assert f'{set_numeric!r}' == "True", "set_numeric"
assert f'{set_miss!r}' == "False", "set_miss"
assert f'{set_not_in!r}' == "True", "set_not_in"
assert f'{text_character!r}' == "True", "text_character"
assert f'{text_substring!r}' == "True", "text_substring"
assert f'{text_surrogate!r}' == "True", "text_surrogate"
assert f'{text_empty!r}' == "True", "text_empty"
assert f'{text_not_in!r}' == "True", "text_not_in"
assert f'{bytes_integer!r}' == "True", "bytes_integer"
assert f'{bytes_bool!r}' == "True", "bytes_bool"
assert f'{bytes_subsequence!r}' == "True", "bytes_subsequence"
assert f'{bytes_empty!r}' == "True", "bytes_empty"
assert f'{bytes_not_in!r}' == "True", "bytes_not_in"
# ---
# case: destructuring assignment
first, second = (1, 2)
[third, fourth] = [3, 4]
left, (middle, right) = [5, (6, 7)]
assert f'{first!r}' == "1", "first"
assert f'{second!r}' == "2", "second"
assert f'{third!r}' == "3", "third"
assert f'{fourth!r}' == "4", "fourth"
assert f'{left!r}' == "5", "left"
assert f'{middle!r}' == "6", "middle"
assert f'{right!r}' == "7", "right"
# ---
# case: extended destructuring assignment
first, *middle, last = (1, 2, 3, 4)
head, *tail = [5, 6, 7]
*prefix, end = (8, 9)
only, *empty = [10]
(left, *center), right = [(11, 12, 13), 14]
[list_left, *list_middle, list_right] = [15, 16, 17, 18]
assert f'{first!r}' == "1", "first"
assert f'{middle!r}' == "[2, 3]", "middle"
assert f'{last!r}' == "4", "last"
assert f'{head!r}' == "5", "head"
assert f'{tail!r}' == "[6, 7]", "tail"
assert f'{prefix!r}' == "[8]", "prefix"
assert f'{end!r}' == "9", "end"
assert f'{only!r}' == "10", "only"
assert f'{empty!r}' == "[]", "empty"
assert f'{left!r}' == "11", "left"
assert f'{center!r}' == "[12, 13]", "center"
assert f'{right!r}' == "14", "right"
assert f'{list_left!r}' == "15", "list_left"
assert f'{list_middle!r}' == "[16, 17]", "list_middle"
assert f'{list_right!r}' == "18", "list_right"
# ---
# case: list comprehensions
outer = (1, 2, 3)
values = [outer * 10 for outer in outer if outer != 2]
pairs = [(1, 2), (0, 3), (4, 5)]
sums = [left + right for left, right in pairs if left]
nested = [(left, right) for left in (1, 2) for right in (3, 4) if left + right != 5]
assert f'{values!r}' == "[10, 30]", "values"
assert f'{outer!r}' == "(1, 2, 3)", "outer"
assert f'{sums!r}' == "[3, 9]", "sums"
assert f'{nested!r}' == "[(1, 3), (2, 4)]", "nested"
# ---
# case: list comprehension closures and assignment expressions
def build(groups, offset):
    last = 0
    values = [item + offset for group in groups if group for item in group if (last := item)]
    return values, last

result = build(((1, 2), (), (3,)), 10)
assert f'{result!r}' == "([11, 12, 13], 3)", "result"
# ---
# case: dictionary item assignment and deletion
mapping = {'first': 1, 'second': 2}
mapping['first'] = 10
mapping['third'] = 3
mapping[True] = 'bool'
mapping[1] = 'integer'
mapping[(1, 2)] = 'pair'
del mapping['second']
mapping['second'] = 20
nested = {'inner': {}}
nested['inner']['value'] = 7
del nested['inner']['value']
want_mapping = "{'first': 10, 'third': 3, True: 'integer', (1, 2): 'pair', 'second': 20}"
assert f'{mapping!r}' == want_mapping
assert f'{nested!r}' == "{'inner': {}}"
