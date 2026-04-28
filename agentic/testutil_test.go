package agentic

import (
	"encoding/json"
	"strconv"
)

func itoa(i int32) string { return strconv.Itoa(int(i)) }

func jsonStrArr(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	out, _ := json.Marshal(s)
	return string(out)
}
