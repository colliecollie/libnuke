package filter_test

import (
	"fmt"

	"github.com/colliecollie/libnuke/pkg/types"
)

type TestResource struct {
	Props types.Properties
}

func (t *TestResource) GetProperty(key string) (string, error) {
	if key == "no_stringer" {
		return "", fmt.Errorf("does not support legacy IDs")
	} else if key == "no_properties" {
		return "", fmt.Errorf("does not support custom properties")
	}

	return "testing", nil
}

func (t *TestResource) Properties() types.Properties {
	return t.Props
}
