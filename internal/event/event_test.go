package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBatch_Add(t *testing.T) {
	file := "/Users/someUser/Projects/someProject/tmp/thing.go"
	operation := Create

	e := &Batch{Events: map[string]Operation{}}
	e.Add(file, operation)

	assert.Equal(t, 1, len(e.Events), "e.Events length is not equal to the expected length")
	assert.Equal(t, operation, e.Events[file], "unexpected operation found")
}
