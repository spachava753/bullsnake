# case: type aliases are not callable
# error: TypeError
# message: "'typing.TypeAliasType' object is not callable"
type Alias = int
Alias()
# ---
# case: missing type alias value names raise lazily
# error: NameError
# message: "name 'missing_alias_value' is not defined"
type MissingAlias = missing_alias_value
MissingAlias.__value__
# ---
# case: generic alias parameters do not leak
# error: NameError
# message: "name 'T' is not defined"
type Hidden[T] = T
T
# ---
# case: missing generic alias bounds raise on access
# error: NameError
# message: "name 'missing_bound' is not defined"
type MissingBound[T: missing_bound] = T
MissingBound.__type_params__[0].__bound__
# ---
# case: missing generic alias defaults raise on access
# error: NameError
# message: "name 'missing_default' is not defined"
type MissingDefault[T = missing_default] = T
MissingDefault.__type_params__[0].__default__
# ---
# case: variadic generic alias parameters do not leak
# error: NameError
# message: "name 'Ts' is not defined"
type HiddenVariadic[*Ts, **P] = (Ts, P)
Ts
