package output

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type Output interface {
	Write(map[string]interface{}, map[string]error) error
}

type OutputKey int

const (
	Console OutputKey = iota + 1
	JSON
	CSV
	HTML
)

var supportedFormats = []string{"json", "csv", "html"}

func SupportedFormats() []string {
	out := make([]string, len(supportedFormats))
	copy(out, supportedFormats)
	return out
}

func SupportedFormatsString() string {
	items := make([]string, len(supportedFormats))
	for i, f := range supportedFormats {
		items[i] = fmt.Sprintf("%s (.%s)", f, f)
	}
	return strings.Join(items, ", ")
}

func GetOutput(o OutputKey, w io.Writer) Output {
	switch o {
	case Console:
		return NewConsoleOutput(w)
	case JSON:
		return NewJSONOutput(w)
	case CSV:
		return NewCSVOutput(w)
	case HTML:
		return NewHTMLOutput(w)
	}
	return nil
}

func OutputKeyFromPath(path string) (OutputKey, error) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	formatsList := SupportedFormatsString()

	switch ext {
	case "json":
		return JSON, nil
	case "csv":
		return CSV, nil
	case "html", "htm":
		return HTML, nil
	default:
		if ext == "" {
			return 0, fmt.Errorf("no file extension found in output path %q. Supported output formats: %s", path, formatsList)
		}
		return 0, fmt.Errorf("unsupported output format: .%s. Supported output formats: %s", ext, formatsList)
	}
}
