package utils

import "encoding/json"

// ToJSON serialize interface to json
func ToJSON(v interface{}) string {
	if v == nil {
		return ""
	}

	b, _ := json.Marshal(v)

	return string(b)
}

func BoolToStr(value bool) string {
	if value {
		return "true"
	}

	return "false"
}
