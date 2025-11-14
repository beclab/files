package serialization

import (
	"io"
	"reflect"
)

type ISerializer interface {
	// DeserializeFromStream deserializes XML from a stream into an object of the given type.
	// The type is typically a pointer to a struct.
	DeserializeFromStream(t reflect.Type, stream io.Reader) (interface{}, error)

	// SerializeToStream serializes an object to XML and writes it to a stream.
	SerializeToStream(obj interface{}, stream io.Writer) error

	// SerializeToFile serializes an object to XML and writes it to a file.
	SerializeToFile(obj interface{}, file string) error

	// DeserializeFromFile deserializes XML from a file into an object of the given type.
	// The type is typically a pointer to a struct.
	DeserializeFromFile(t reflect.Type, file string) (interface{}, error)

	// DeserializeFromBytes deserializes XML from a byte slice into an object of the given type.
	// The type is typically a pointer to a struct.
	DeserializeFromBytes(t reflect.Type, buffer []byte) (interface{}, error)
}
