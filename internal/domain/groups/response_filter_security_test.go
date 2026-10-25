package groups

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidateUniqueJSONKeysHandlesManyUniqueKeys(t *testing.T) {
	const keyCount = 10_000
	var body strings.Builder
	body.WriteByte('{')
	for index := range keyCount {
		if index > 0 {
			body.WriteByte(',')
		}
		fmt.Fprintf(&body, "%q:%d", fmt.Sprintf("field_%d", index), index)
	}
	body.WriteByte('}')

	if err := validateUniqueJSONKeys([]byte(body.String())); err != nil {
		t.Fatalf("validate large object: %v", err)
	}
}
