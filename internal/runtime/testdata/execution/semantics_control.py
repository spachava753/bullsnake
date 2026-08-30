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
