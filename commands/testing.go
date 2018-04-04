package commands

// A MockEmitter satisfies the ValueEmitter interface and records its calls.
type MockEmitter struct {
	emitterFunc func(value interface{}) error
	captures    *[]interface{}
}

// NewMockEmitter creates a MockEmitter from the provided emitFunc.
func NewMockEmitter(emitFunc func(interface{}) error) *MockEmitter {
	return &MockEmitter{
		emitterFunc: emitFunc,
		captures:    &[]interface{}{},
	}
}

func (ce MockEmitter) emit(value interface{}) error {
	*ce.captures = append(*ce.captures, value)
	return ce.emitterFunc(value)
}

func (ce MockEmitter) calls() []interface{} {
	return *ce.captures
}
