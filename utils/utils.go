package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/version"
	"k8s.io/apimachinery/pkg/util/wait"
)

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

func StrToBool(value string) bool {
	return strings.ToLower(value) == "true"
}

func StrToInt(value string) int {
	if n, e := strconv.Atoi(value); e != nil {
		return 0
	} else {
		return n
	}
}

func UserAgent() string {
	return fmt.Sprintf("vmware-desktop-autoscaler-utility/%s/go", version.VERSION)
}

func PollImmediate(interval, timeout time.Duration, condition wait.ConditionFunc) error {
	if timeout == 0 {
		return wait.PollImmediateInfinite(interval, condition)
	} else {
		return wait.PollImmediate(interval, timeout, condition)
	}
}

// FileExists Check if FileExists
func FileExists(name string) bool {
	if len(name) == 0 {
		return false
	}

	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}
