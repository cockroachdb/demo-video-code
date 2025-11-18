package models

import (
	"encoding/json"
	"strconv"
	"strings"
)

type VectorString []float64

func (v *VectorString) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	str = strings.TrimPrefix(str, "[")
	str = strings.TrimSuffix(str, "]")

	parts := strings.Split(str, ",")

	result := make([]float64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return err
		}
		result = append(result, f)
	}

	*v = result
	return nil
}
