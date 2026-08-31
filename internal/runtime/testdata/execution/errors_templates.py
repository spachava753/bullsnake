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
