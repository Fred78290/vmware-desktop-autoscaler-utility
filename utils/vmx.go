package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type VMXMap struct {
	vmx  map[string]string
	keys map[string]string
}

func (vmx *VMXMap) Has(key string) bool {
	_, found := vmx.keys[strings.ToLower(key)]

	return found
}

func (vmx *VMXMap) Get(key string) string {
	if real, found := vmx.keys[strings.ToLower(key)]; found {
		return vmx.vmx[real]
	}

	return ""
}

func (vmx *VMXMap) Delete(key string) string {
	if real, found := vmx.keys[strings.ToLower(key)]; found {
		delete(vmx.vmx, real)
	}

	return ""
}

func (vmx *VMXMap) Keys() []string {
	result := make([]string, 0, len(vmx.keys))

	for k, _ := range vmx.keys {
		result = append(result, k)
	}

	return result
}

func (vmx *VMXMap) Set(key, value string) {
	if real, found := vmx.keys[strings.ToLower(key)]; found {
		vmx.vmx[real] = value
	} else {
		lower := strings.ToLower(key)

		vmx.keys[lower] = key
		vmx.vmx[key] = value
	}
}

func (vmx *VMXMap) Load(vmxpath string) error {

	if file, err := os.Open(vmxpath); err != nil {
		return err
	} else {
		defer file.Close()

		fileScanner := bufio.NewScanner(file)

		fileScanner.Split(bufio.ScanLines)

		for fileScanner.Scan() {
			line := fileScanner.Text()

			if !strings.HasPrefix(line, ".encoding") {
				segments := strings.Split(line, "=")
				key := strings.TrimSpace(segments[0])
				value := strings.Trim(strings.TrimSpace(segments[1]), "\"")

				vmx.vmx[key] = value
				vmx.keys[strings.ToLower(key)] = key
			}
		}

		return nil
	}
}

func (vmx *VMXMap) Save(vmxpath string) error {
	if file, err := os.OpenFile(vmxpath, os.O_WRONLY, 0644); err != nil {
		return err
	} else {
		datawriter := bufio.NewWriter(file)

		datawriter.WriteString(".encoding = \"UTF-8\"\n")

		for k, v := range vmx.vmx {
			datawriter.WriteString(fmt.Sprintf("%s = \"%s\"\n", k, v))
		}

		datawriter.Flush()
		file.Close()
	}

	return nil
}

func LoadVMX(vmxpath string) (*VMXMap, error) {
	vmx := &VMXMap{
		keys: make(map[string]string),
		vmx:  make(map[string]string),
	}

	return vmx, vmx.Load(vmxpath)
}
