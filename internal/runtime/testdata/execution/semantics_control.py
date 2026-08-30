# Runtime execution cases for control.
# case: boolean and conditional control flow
false_and = 0 and missing
true_and = 5 and 9
true_or = 5 or missing
false_or = 0 or 9
true_conditional = 10 if 'x' else missing
false_conditional = missing if '' else 20
if None:
    branch = missing
else:
    branch = 30
if 1:
    second_branch = 40
else:
    second_branch = missing
assert f'{false_and!r}' == "0", "false_and"
assert f'{true_and!r}' == "9", "true_and"
assert f'{true_or!r}' == "5", "true_or"
assert f'{false_or!r}' == "9", "false_or"
assert f'{true_conditional!r}' == "10", "true_conditional"
assert f'{false_conditional!r}' == "20", "false_conditional"
assert f'{branch!r}' == "30", "branch"
assert f'{second_branch!r}' == "40", "second_branch"
# ---
# case: comparisons
equal = 2 == 2
not_equal = 2 != 3
less = 2 < 3
less_equal = 2 <= 2
greater = 3 > 2
greater_equal = 3 >= 3
bool_integer_equal = True == 1
mixed_numeric_equal = 1 == 1.0
mixed_numeric_order = 1 < 1.5
complex_zero_equal = 0j == 0
text_order = 'alpha' < 'beta'
bytes_order = b'beta' > b'alpha'
none_equal = None == None
different_types = None != 1
same_identity = None is None
different_identity = None is not ...
true_chain = 1 < 2 < 3
false_chain = 1 < 3 < 2
short_chain = 3 < 2 < missing
assert f'{equal!r}' == "True", "equal"
assert f'{not_equal!r}' == "True", "not_equal"
assert f'{less!r}' == "True", "less"
assert f'{less_equal!r}' == "True", "less_equal"
assert f'{greater!r}' == "True", "greater"
assert f'{greater_equal!r}' == "True", "greater_equal"
assert f'{bool_integer_equal!r}' == "True", "bool_integer_equal"
assert f'{mixed_numeric_equal!r}' == "True", "mixed_numeric_equal"
assert f'{mixed_numeric_order!r}' == "True", "mixed_numeric_order"
assert f'{complex_zero_equal!r}' == "True", "complex_zero_equal"
assert f'{text_order!r}' == "True", "text_order"
assert f'{bytes_order!r}' == "True", "bytes_order"
assert f'{none_equal!r}' == "True", "none_equal"
assert f'{different_types!r}' == "True", "different_types"
assert f'{same_identity!r}' == "True", "same_identity"
assert f'{different_identity!r}' == "True", "different_identity"
assert f'{true_chain!r}' == "True", "true_chain"
assert f'{false_chain!r}' == "False", "false_chain"
assert f'{short_chain!r}' == "False", "short_chain"
# ---
# case: passing assertion
assert True
answer = 42
assert answer == 42
# ---
# case: while loops
count = 5
total = 0
while count:
    total = total + count
    count = count - 1
else:
    completed = 1
break_count = 3
while break_count:
    break_count = break_count - 1
    if break_count:
        continue
    break
else:
    skipped_else = missing
after_break = break_count
assert count == 0
assert total == 15
assert completed == 1
assert break_count == 0
assert after_break == 0
# ---
# case: tuple and list for loops
total = 0
for value in (1, 2, 3):
    total = total + value
else:
    completed = total
continued_total = 0
for value in [4, 5, 6]:
    if value == 5:
        continue
    continued_total = continued_total + value
else:
    continued = 1
for stopped in (7, 8, 9):
    if stopped == 8:
        break
else:
    skipped_else = missing
after_break = stopped
for absent in ():
    missing
else:
    empty_else = 11
pair_total = 0
for left, right in [(1, 2), (3, 4)]:
    pair_total = pair_total + left + right
nested_total = 0
for outer in (1, 2):
    for inner in [10, 20]:
        nested_total = nested_total + outer + inner
assert total == 6
assert completed == 6
assert continued_total == 10
assert continued == 1
assert after_break == 8
assert empty_else == 11
assert pair_total == 10
assert nested_total == 66
# ---
# case: dictionary and set for loops
mapping = {'first': 1, 'second': 2}
mapping_total = 0
mapping_position = 1
mapping_order = 0
for key in mapping:
    mapping_total = mapping_total + mapping[key]
    if key == 'first':
        mapping_order = mapping_order + mapping_position
    else:
        mapping_order = mapping_order + mapping_position * 10
    mapping_position = mapping_position + 1
else:
    mapping_complete = True
set_total = 0
set_count = 0
for value in {3, 1, 2, 1}:
    set_total = set_total + value
    set_count = set_count + 1
for absent in {*()}:
    missing
else:
    empty_complete = True
pairs = {(4, 5): 1, (6, 7): 2}
pair_total = 0
for left, right in pairs:
    pair_total = pair_total + left + right
for key in mapping:
    mapping[key] = mapping[key] + 10
assert f'{mapping!r}' == "{'first': 11, 'second': 12}"
assert mapping_total == 3
assert mapping_order == 21
assert mapping_complete is True
assert set_total == 6
assert set_count == 3
assert empty_complete is True
assert pair_total == 22
# ---
# case: string and bytes for loops
text_seen = {}
text_count = 0
for character in 'A\u00e9\U0001f40d\ud800':
    text_seen[text_count] = character
    text_count = text_count + 1
byte_seen = {}
byte_count = 0
byte_total = 0
for octet in b'\x00A\xff':
    byte_seen[byte_count] = octet
    byte_count = byte_count + 1
    byte_total = byte_total + octet
for absent_text in '':
    missing
else:
    empty_text_complete = True
for absent_byte in b'':
    missing
else:
    empty_bytes_complete = True
assert f'{text_seen!r}' == "{0: 'A', 1: 'é', 2: '🐍', 3: '\\ud800'}"
assert text_count == 4
assert f'{byte_seen!r}' == "{0: 0, 1: 65, 2: 255}"
assert byte_count == 3
assert byte_total == 320
assert empty_text_complete is True
assert empty_bytes_complete is True
