package output

import (
	"encoding/json"
	"io"
)

type JSONOutput struct {
	w io.Writer
}

func NewJSONOutput(w io.Writer) *JSONOutput {
	return &JSONOutput{w: w}
}

func (o *JSONOutput) Write(result map[string]interface{}, errs map[string]error) error {
	output := map[string]interface{}{
		"results": result,
		"errors":  errs,
	}

	encoder := json.NewEncoder(o.w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
