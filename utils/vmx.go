package utils

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

type VMXMap struct {
	headline string
	vmx      map[string]string
	keys     map[string]string
}

func (vmx *VMXMap) Cleanup(removeCard bool) {
	vmx.Delete("instance-id")
	vmx.Delete("hostname")
	vmx.Delete("seedfrom")
	vmx.Delete("public-keys")
	vmx.Delete("user-data")
	vmx.Delete("password")
	vmx.Delete("vmxstats.filename")

	// Remove ethernet cards & old guest info
	for _, key := range vmx.Keys() {
		if (removeCard && strings.HasPrefix(key, "ethernet")) || strings.HasPrefix(key, "guestinfo") {
			vmx.Delete(key)
		}
	}
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

func (vmx *VMXMap) sortedKeys() []string {
	result := make([]string, 0, len(vmx.keys))

	for k := range vmx.vmx {
		result = append(result, k)
	}

	sort.Strings(result)

	return result
}

func (vmx *VMXMap) Keys() []string {
	result := make([]string, 0, len(vmx.keys))

	for k := range vmx.keys {
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

		count := 0

		for fileScanner.Scan() {
			line := fileScanner.Text()

			if strings.HasPrefix(line, "#!") {
				if count == 0 {
					vmx.headline = line
				}
			} else if !strings.HasPrefix(line, ".encoding") && !strings.HasPrefix(line, "#") {
				offset := strings.Index(line, "=")

				if offset >= 0 {
					key := strings.TrimSpace(line[:offset-1])
					value := strings.Trim(strings.TrimSpace(line[offset+1:]), "\"")

					vmx.vmx[key] = value
					vmx.keys[strings.ToLower(key)] = key
				} else {
					fmt.Printf("Drop line:%s\n", line)
				}
			}

			count++
		}

		return nil
	}
}

func (vmx *VMXMap) Save(vmxpath string) error {
	if file, err := os.OpenFile(vmxpath, os.O_WRONLY|os.O_TRUNC, 0644); err != nil {
		return err
	} else {
		datawriter := bufio.NewWriter(file)

		if vmx.headline != "" {
			datawriter.WriteString(fmt.Sprintf("%s\n", vmx.headline))
		}

		datawriter.WriteString(".encoding = \"UTF-8\"\n")

		sortedKeys := vmx.sortedKeys()

		for _, k := range sortedKeys {
			datawriter.WriteString(fmt.Sprintf("%s = \"%s\"\n", k, vmx.vmx[k]))
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
