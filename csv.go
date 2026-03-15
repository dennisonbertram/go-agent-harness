package main

import (
	"encoding/csv"
	"io"
	"strings"
)

// ParseCSV parses a CSV string with the first row as headers and returns a slice of maps
func ParseCSV(s string) []map[string]string {
	r := csv.NewReader(strings.NewReader(s))
	r.FieldsPerRecord = -1 // allow variable record length

	headers, err := r.Read()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return nil
	}
	var out []map[string]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed lines
		}
		// Skip empty lines
		if len(rec) == 0 || (len(rec) == 1 && len(strings.TrimSpace(rec[0])) == 0) {
			continue
		}
		tuple := make(map[string]string)
		for i, v := range rec {
			if i < len(headers) {
				tuple[headers[i]] = v
			}
		}
		out = append(out, tuple)
	}
	return out
}
