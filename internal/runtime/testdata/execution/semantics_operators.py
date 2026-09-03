# Runtime execution cases for operators.
# case: augmented integer assignments
value = 20
value += 5
value *= 2
value -= 8
value //= 3
value %= 5
value <<= 3
value >>= 2
value |= 2
value ^= 3
value &= 7
class Box:
    value = 10
Box.value += 5
class_value = Box.value
box = Box()
box.value = 20
box.value += 2
instance_value = box.value
mapping = {'count': 40}
mapping['count'] += 2
mapping_value = mapping['count']
assert f'{value!r}' == "1", "value"
assert f'{class_value!r}' == "15", "class_value"
assert f'{instance_value!r}' == "22", "instance_value"
assert f'{mapping_value!r}' == "42", "mapping_value"
# ---
# case: unary operators
positive_integer = +7
positive_bool = +True
negative_integer = -7
negative_bool = -True
inverted_integer = ~7
inverted_bool = ~False
negative_float = -1.25
positive_imaginary = +2j
negative_imaginary = -2j
not_none = not None
not_false = not False
not_zero = not 0
not_zero_float = not 0.0
not_zero_complex = not 0j
not_empty_text = not ''
not_empty_bytes = not b''
not_ellipsis = not ...
not_nonzero = not 1
not_text = not 'x'
assert f'{positive_integer!r}' == "7", "positive_integer"
assert f'{positive_bool!r}' == "1", "positive_bool"
assert f'{negative_integer!r}' == "-7", "negative_integer"
assert f'{negative_bool!r}' == "-1", "negative_bool"
assert f'{inverted_integer!r}' == "-8", "inverted_integer"
assert f'{inverted_bool!r}' == "-1", "inverted_bool"
assert f'{negative_float!r}' == "-1.25", "negative_float"
assert f'{positive_imaginary!r}' == "2j", "positive_imaginary"
assert f'{negative_imaginary!r}' == "(-0-2j)", "negative_imaginary"
assert f'{not_none!r}' == "True", "not_none"
assert f'{not_false!r}' == "True", "not_false"
assert f'{not_zero!r}' == "True", "not_zero"
assert f'{not_zero_float!r}' == "True", "not_zero_float"
assert f'{not_zero_complex!r}' == "True", "not_zero_complex"
assert f'{not_empty_text!r}' == "True", "not_empty_text"
assert f'{not_empty_bytes!r}' == "True", "not_empty_bytes"
assert f'{not_ellipsis!r}' == "False", "not_ellipsis"
assert f'{not_nonzero!r}' == "False", "not_nonzero"
assert f'{not_text!r}' == "False", "not_text"
# ---
# case: basic integer operations
addition = 40 + 2
bool_addition = True + 2
subtraction = 1000000000000000000000000000000 - 1
multiplication = -12 * 11
bool_multiplication = False * 99
bitwise_or = 10 | 5
bitwise_xor = 10 ^ 3
bitwise_and = 10 & 6
assert f'{addition!r}' == "42", "addition"
assert f'{bool_addition!r}' == "3", "bool_addition"
assert f'{subtraction!r}' == "999999999999999999999999999999", "subtraction"
assert f'{multiplication!r}' == "-132", "multiplication"
assert f'{bool_multiplication!r}' == "0", "bool_multiplication"
assert f'{bitwise_or!r}' == "15", "bitwise_or"
assert f'{bitwise_xor!r}' == "9", "bitwise_xor"
assert f'{bitwise_and!r}' == "2", "bitwise_and"
# ---
# case: floor division and modulo
positive_floor = 5 // 2
left_negative_floor = -5 // 2
right_negative_floor = 5 // -2
both_negative_floor = -5 // -2
left_negative_modulo = -5 % 2
right_negative_modulo = 5 % -2
both_negative_modulo = -5 % -2
large_floor = 1000000000000000000000000000000 // 3
large_modulo = 1000000000000000000000000000000 % 3
bool_floor = True // True
bool_modulo = False % True
assert f'{positive_floor!r}' == "2", "positive_floor"
assert f'{left_negative_floor!r}' == "-3", "left_negative_floor"
assert f'{right_negative_floor!r}' == "-3", "right_negative_floor"
assert f'{both_negative_floor!r}' == "2", "both_negative_floor"
assert f'{left_negative_modulo!r}' == "1", "left_negative_modulo"
assert f'{right_negative_modulo!r}' == "-1", "right_negative_modulo"
assert f'{both_negative_modulo!r}' == "-1", "both_negative_modulo"
assert f'{large_floor!r}' == "333333333333333333333333333333", "large_floor"
assert f'{large_modulo!r}' == "1", "large_modulo"
assert f'{bool_floor!r}' == "1", "bool_floor"
assert f'{bool_modulo!r}' == "0", "bool_modulo"
# ---
# case: true division
integer_division = 7 / 2
float_division = 7.5 / 2
reverse_float_division = 7 / 2.0
bool_division = True / 2
assert integer_division == 3.5
assert float_division == 3.75
assert reverse_float_division == 3.5
assert bool_division == 0.5
# ---
# case: integer shifts
left_shift = 5 << 3
right_shift = 40 >> 3
negative_left = -5 << 2
negative_right = -5 >> 1
bool_left = True << 4
bool_right = 8 >> True
huge_right = 1 >> 10000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
huge_negative_right = -1 >> 10000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
huge_zero_left = 0 << 10000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
assert left_shift == 40
assert right_shift == 5
assert negative_left == -20
assert negative_right == -3
assert bool_left == 16
assert bool_right == 4
assert huge_right == 0
assert huge_negative_right == -1
assert huge_zero_left == 0
