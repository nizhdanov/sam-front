package opt

import "encoding/json"

type Field[T any] struct {
	Set   bool
	Value *T
}

func Of[T any](v T) Field[T] { return Field[T]{Set: true, Value: &v} }

func Null[T any]() Field[T] { return Field[T]{Set: true} }

func (f *Field[T]) UnmarshalJSON(b []byte) error {
	f.Set, f.Value = true, nil
	if string(b) == "null" {
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	f.Value = &v
	return nil
}
