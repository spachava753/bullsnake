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
