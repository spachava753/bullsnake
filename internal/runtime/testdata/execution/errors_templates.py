# Template string evaluation errors.
# case: missing template interpolation name
# error: NameError
# message: "name 'missing_template_name' is not defined"
value = t'{missing_template_name}'
# ---
# case: template plus string
# error: TypeError
# message: "can only concatenate string.templatelib.Template (not \"str\") to string.templatelib.Template"
result = t'left' + 'right'
# ---
# case: string plus template
# error: TypeError
# message: "can only concatenate str (not \"string.templatelib.Template\") to str"
result = 'left' + t'right'
# ---
# case: template constructor rejects keyword arguments
# error: TypeError
# message: "Template.__new__ only accepts *args arguments"
from string.templatelib import Template
result = Template(value='text')
# ---
# case: template constructor rejects other values
# error: TypeError
# message: "Template.__new__ *args need to be of type 'str' or 'Interpolation', got int"
from string.templatelib import Template
result = Template(1)
# ---
# case: interpolation expression must be a string
# error: TypeError
# message: "Interpolation() argument 'expression' must be str, not int"
from string.templatelib import Interpolation
result = Interpolation('value', 1)
# ---
# case: interpolation conversion must be a string
# error: TypeError
# message: "Interpolation() argument 'conversion' must be str, not int"
from string.templatelib import Interpolation
result = Interpolation('value', 'value', 1)
# ---
# case: interpolation conversion must be known
# error: ValueError
# message: "Interpolation() argument 'conversion' must be one of 's', 'a' or 'r'"
from string.templatelib import Interpolation
result = Interpolation('value', 'value', 'z')
# ---
# case: interpolation format must be a string
# error: TypeError
# message: "Interpolation() argument 'format_spec' must be str, not int"
from string.templatelib import Interpolation
result = Interpolation('value', 'value', None, 1)
