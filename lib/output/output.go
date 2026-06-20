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
	switch ext {
	case "json":
		return JSON, nil
	case "csv":
		return CSV, nil
	case "html", "htm":
		return HTML, nil
	default:
		return 0, fmt.Errorf("unsupported output format: .%s. Supported formats: %s", ext, strings.Join(supportedFormats, ", "))
	}
}
