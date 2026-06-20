package output

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

type HTMLOutput struct {
	w io.Writer
}

func NewHTMLOutput(w io.Writer) *HTMLOutput {
	return &HTMLOutput{w: w}
}

func (o *HTMLOutput) Write(result map[string]interface{}, errs map[string]error) error {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>PhoneInfoga Scan Results</title>
<style>
body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
h1 { color: #333; }
h2 { color: #555; border-bottom: 1px solid #ddd; padding-bottom: 5px; }
table { border-collapse: collapse; width: 100%; margin-bottom: 20px; }
th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
th { background-color: #4CAF50; color: white; }
tr:nth-child(even) { background-color: #f2f2f2; }
.errors { background-color: #ffebee; border: 1px solid #ef9a9a; padding: 10px; margin-top: 20px; }
.errors h2 { color: #c62828; }
.error-item { color: #c62828; margin: 5px 0; }
</style>
</head>
<body>
<h1>PhoneInfoga Scan Results</h1>
`)

	for _, name := range getSortedResultKeys(result) {
		res := result[name]
		if res == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("<h2>%s</h2>\n", escapeHTML(name)))
		sb.WriteString("<table><tr><th>Field</th><th>Value</th></tr>\n")
		o.renderFields(&sb, res, "")
		sb.WriteString("</table>\n")
	}

	if len(errs) > 0 {
		sb.WriteString(`<div class="errors"><h2>Errors</h2>`)
		for _, name := range getSortedErrorKeys(errs) {
			sb.WriteString(fmt.Sprintf(`<div class="error-item"><strong>%s</strong>: %s</div>`, escapeHTML(name), escapeHTML(errs[name].Error())))
		}
		sb.WriteString("</div>\n")
	}

	sb.WriteString("</body>\n</html>")

	_, err := fmt.Fprint(o.w, sb.String())
	return err
}

func (o *HTMLOutput) renderFields(sb *strings.Builder, val interface{}, prefix string) {
	reflectType := reflect.TypeOf(val)
	reflectValue := reflect.ValueOf(val)

	if reflectValue.Kind() == reflect.Slice {
		for i := 0; i < reflectValue.Len(); i++ {
			item := reflectValue.Index(i)
			if item.Kind() == reflect.Ptr {
				item = item.Elem()
			}
			o.renderFields(sb, item.Interface(), prefix)
		}
		return
	}

	for i := 0; i < reflectType.NumField(); i++ {
		field, ok := reflectType.Field(i).Tag.Lookup("json")
		if !ok || field == "-" {
			continue
		}
		fieldName := strings.Split(field, ",")[0]
		if fieldName == "" {
			continue
		}

		displayName := fieldName
		if prefix != "" {
			displayName = prefix + "." + fieldName
		}

		switch reflectValue.Field(i).Kind() {
		case reflect.String:
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>\n", escapeHTML(displayName), escapeHTML(reflectValue.Field(i).String())))
		case reflect.Bool:
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%v</td></tr>\n", escapeHTML(displayName), reflectValue.Field(i).Bool()))
		case reflect.Int, reflect.Int32, reflect.Int64:
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>\n", escapeHTML(displayName), reflectValue.Field(i).Int()))
		case reflect.Struct:
			o.renderFields(sb, reflectValue.Field(i).Interface(), displayName)
		case reflect.Slice:
			o.renderFields(sb, reflectValue.Field(i).Interface(), displayName)
		}
	}
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
