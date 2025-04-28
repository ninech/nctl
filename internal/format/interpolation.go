package format

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"
)

// interpolationRegex is the regex to find if a given string contains variables
// for interpolation
var interpolationRegex = regexp.MustCompile(`(\$\$)|((?:\${([[:alpha:]_][[:word:]]*))(?:=([^}]+))?})|(\$)|([^$]+)`)

// interpolate interpolates the given string s with variables from vars
// The [upstream
// function](https://github.com/alecthomas/kong/blob/v0.8.0/interpolate.go#L22)
// was sadly not exported, so we had to copy it.
func interpolate(s string, vars kong.Vars) (string, error) {
	out := ""
	matches := interpolationRegex.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return s, nil
	}
	for _, match := range matches {
		if dollar := match[1]; dollar != "" {
			out += "$"
		} else if name := match[3]; name != "" {
			value, ok := vars[name]
			if !ok {
				// No default value.
				if match[4] == "" {
					return "", fmt.Errorf("undefined variable ${%s}", name)
				}
				value = match[4]
			}
			out += value
		} else {
			out += match[0]
		}
	}
	return out, nil
}

// InterpolateFlagPlaceholders will return a function which walks the whole kong
// model and interpolates variables in placeholders in flags.
func InterpolateFlagPlaceholders(vars kong.Vars) func(*kong.Kong) error {
	var walkNode func(n *kong.Node) error
	walkNode = func(n *kong.Node) error {
		var err error
		if n == nil {
			return nil
		}
		for i := range n.Flags {
			if n.Flags[i] == nil {
				continue
			}
			if n.Flags[i].PlaceHolder, err = interpolate(n.Flags[i].PlaceHolder, vars); err != nil {
				return fmt.Errorf("error when interpolating placeholder tag of flag %q: %w", n.Flags[i].Name, err)
			}
		}
		// we are now calling ourselves for all child nodes
		for i := range n.Children {
			if err := walkNode(n.Children[i]); err != nil {
				return err
			}
		}
		return nil
	}
	return func(k *kong.Kong) error {
		if k.Model == nil {
			return errors.New("no kong model found")
		}
		return walkNode(k.Model.Node)
	}
}

// makeInterpolatedType takes any “template” value whose type is a struct (or pointer to struct).
// It reads each struct field's raw `reflect.StructTag`, does a strings.ReplaceAll on every
// "${key}" → vars[key], and then returns a brand new reflect.Type via reflect.StructOf.
// For example, if your original type is:
//
//	type FooTemplate struct {
//	   Name string `arg:"" predictor:"${name_predictor}" help:"Name of the resource. ${name_help_note}" default:"${name_default}"`
//	}
//
// and you call
//
//	func (Cmd) KongVars() kong.Vars {
//			return kong.Vars{
//				"name_predictor": "resource_name",
//				"name_help_note": "",
//				"name_default":   "",
//			}
//		}
//
// then the returned reflect.Type is equivalent to:
//
//	struct {
//	    Name string `arg:"" predictor:"resource_name" help:"Name of the resource." default:""`
//	}
func MakeInterpolatedType(template interface{}, vars map[string]string) (reflect.Type, error) {
	origType := reflect.TypeOf(template)
	if origType.Kind() == reflect.Ptr {
		origType = origType.Elem()
	}
	if origType.Kind() != reflect.Struct {
		return nil, errors.New("makeInterpolatedType: template must be a struct or pointer-to-struct")
	}

	var fields []reflect.StructField
	for i := 0; i < origType.NumField(); i++ {
		f := origType.Field(i)

		rawTag := string(f.Tag) // e.g. `arg:"" predictor:"${name_predictor}" help:"Name of the resource. ${name_help_note}" default:"${name_default}"`
		newTag := rawTag
		// TODO: remove. quick ugly debugging:
		if strings.Contains(rawTag, "predictor") {
			fmt.Println(rawTag)
		}

		// Replace all occurrences of ${key} with vars[key], if present.
		// If vars[key] is the empty string, we effectively remove that placeholder
		// (e.g. name_predictor="" means predictor:"" so Kong sees “no predictor”).
		for key, repl := range vars {
			placeholder := "${" + key + "}"
			newTag = strings.ReplaceAll(newTag, placeholder, repl)
		}

		f.Tag = reflect.StructTag(newTag)
		fields = append(fields, f)
	}

	// reflect.StructOf builds a brand-new struct type whose fields & tags exactly match “fields”.
	return reflect.StructOf(fields), nil
}
