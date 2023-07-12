package event

import (
	"fmt"
)

type Operation int

func (o Operation) String() string {
	return [...]string{
		"CREATE",
		"DELETE",
		"WRITE",
	}[o]
}

const (
	Create Operation = iota
	Delete
	Write
)

type Batch struct {
	Events map[string]Operation
}

func NewBatch() *Batch {
	return &Batch{
		Events: make(map[string]Operation),
	}
}

func (e *Batch) Paths() []string {
	paths := make([]string, 0, len(e.Events))

	for path := range e.Events {
		paths = append(paths, path)
	}

	return paths
}

func (e *Batch) Add(path string, op Operation) {
	if _, ok := e.Events[path]; ok {
		return
	}

	fmt.Println("thing")

	e.Events[path] = op
}
