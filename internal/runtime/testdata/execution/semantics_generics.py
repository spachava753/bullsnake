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
