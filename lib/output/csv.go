package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"reflect"
	"strings"
)

type CSVOutput struct {
	w io.Writer
}

func NewCSVOutput(w io.Writer) *CSVOutput {
	return &CSVOutput{w: w}
}

func (o *CSVOutput) Write(result map[string]interface{}, errs map[string]error) error {
	csvWriter := csv.NewWriter(o.w)

	if err := csvWriter.Write([]string{"Scanner", "Field", "Value"}); err != nil {
		return err
	}

	for _, name := range getSortedResultKeys(result) {
		res := result[name]
		if res == nil {
			continue
		}
		rows := o.extractFields(res, name, "")
		for _, row := range rows {
			if err := csvWriter.Write(row); err != nil {
				return err
			}
		}
	}

	if len(errs) > 0 {
		for _, name := range getSortedErrorKeys(errs) {
			row := []string{name, "error", errs[name].Error()}
			if err := csvWriter.Write(row); err != nil {
				return err
			}
		}
	}

	csvWriter.Flush()
	return csvWriter.Error()
}

func (o *CSVOutput) extractFields(val interface{}, scannerName, prefix string) [][]string {
	var rows [][]string
	reflectType := reflect.TypeOf(val)
	reflectValue := reflect.ValueOf(val)

	if reflectValue.Kind() == reflect.Slice {
		for i := 0; i < reflectValue.Len(); i++ {
			item := reflectValue.Index(i)
			if item.Kind() == reflect.Ptr {
				item = item.Elem()
			}
			rows = append(rows, o.extractFields(item.Interface(), scannerName, prefix)...)
		}
		return rows
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

		fullName := fieldName
		if prefix != "" {
			fullName = prefix + "." + fieldName
		}

		switch reflectValue.Field(i).Kind() {
		case reflect.String:
			rows = append(rows, []string{scannerName, fullName, reflectValue.Field(i).String()})
		case reflect.Bool:
			rows = append(rows, []string{scannerName, fullName, fmt.Sprintf("%v", reflectValue.Field(i).Bool())})
		case reflect.Int, reflect.Int32, reflect.Int64:
			rows = append(rows, []string{scannerName, fullName, fmt.Sprintf("%d", reflectValue.Field(i).Int())})
		case reflect.Struct:
			rows = append(rows, o.extractFields(reflectValue.Field(i).Interface(), scannerName, fullName)...)
		case reflect.Slice:
			rows = append(rows, o.extractFields(reflectValue.Field(i).Interface(), scannerName, fullName)...)
		}
	}

	return rows
}
