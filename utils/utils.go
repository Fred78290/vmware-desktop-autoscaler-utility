package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/version"
	"k8s.io/apimachinery/pkg/util/wait"
)

func StoreJsonToFile(path string, v interface{}) error {
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644); err != nil {
		return err
	} else {
		defer f.Close()

		_, err = f.WriteString(ToJSON(v))

		return err
	}
}

func LoadJsonFromFile(path string, v interface{}) error {
	if f, err := os.Open(path); err != nil {
		return err
	} else {
		defer f.Close()

		if contents, err := io.ReadAll(f); err != nil {
			return err
		} else {
			return FromJSON(contents, v)
		}
	}
}

// ToJSON serialize interface to json
func ToJSON(v interface{}) string {
	if v == nil {
		return ""
	}

	b, _ := json.Marshal(v)

	return string(b)
}

func FromJSON(data []byte, v interface{}) error {
	if v == nil {
		return fmt.Errorf("recipient is nil")
	}

	return json.Unmarshal(data, v)
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

func MkDir(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(path, os.ModePerm)
	}

	return nil
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
