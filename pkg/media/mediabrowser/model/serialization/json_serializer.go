package serialization

import (
	"io"
	"reflect"
)

type IJsonSerializer interface {
	// DeserializeFromStream deserializes JSON from a stream into an object of the given type.
	// The type is typically a pointer to a struct.
	DeserializeFromStream(t reflect.Type, stream io.Reader) (interface{}, error)

	// SerializeToStream serializes an object to JSON and writes it to a stream.
	SerializeToStream(obj interface{}, stream io.Writer) error

	// SerializeToFile serializes an object to JSON and writes it to a file.
	SerializeToFile(obj interface{}, file string) error

	// DeserializeFromFile deserializes JSON from a file into an object of the given type.
	// The type is typically a pointer to a struct.
	DeserializeFromFile(t reflect.Type, file string) (interface{}, error)

	// DeserializeFromBytes deserializes JSON from a byte slice into an object of the given type.
	// The type is typically a pointer to a struct.
	DeserializeFromBytes(t reflect.Type, buffer []byte) (interface{}, error)
}
