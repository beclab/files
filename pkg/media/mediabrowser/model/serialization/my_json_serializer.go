package serialization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
)

// MyJsonSerializer implements JSON serialization
type MyJsonSerializer struct {
	serializers sync.Map // Cache of type to reflect.Type
}

// NewMyJsonSerializer creates a new MyJsonSerializer
func NewMyJsonSerializer() *MyJsonSerializer {
	return &MyJsonSerializer{}
}

// getSerializer retrieves or caches the type for consistency
func (s *MyJsonSerializer) getSerializer(t reflect.Type) interface{} {
	key := t.String()
	if serializer, ok := s.serializers.Load(key); ok {
		return serializer
	}
	// Cache the type for consistency, though json.Marshal/Unmarshal don't require it
	s.serializers.Store(key, t)
	return t
}

// SerializeToWriter serializes an object to a JSON writer
func (s *MyJsonSerializer) serializeToWriter(obj interface{}, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ") // Equivalent to pretty-printing
	return encoder.Encode(obj)
}

// DeserializeFromStream deserializes from a stream
func (s *MyJsonSerializer) DeserializeFromStream(t reflect.Type, r io.Reader) (interface{}, error) {
	decoder := json.NewDecoder(r)
	v := reflect.New(t).Interface()
	if err := decoder.Decode(v); err != nil {
		return nil, err
	}
	fmt.Printf("DeserializeFromStream json: %+v\n", v)
	return v, nil
}

// SerializeToStream serializes an object to a stream
func (s *MyJsonSerializer) SerializeToStream(obj interface{}, w io.Writer) error {
	// Use a buffer to ensure proper handling
	var buf bytes.Buffer
	if err := s.serializeToWriter(obj, &buf); err != nil {
		return err
	}
	_, err := io.Copy(w, &buf)
	return err
}

// SerializeToFile serializes an object to a file
func (s *MyJsonSerializer) SerializeToFile(obj interface{}, file string) error {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.SerializeToStream(obj, f)
}

// DeserializeFromFile deserializes from a file
func (s *MyJsonSerializer) DeserializeFromFile(t reflect.Type, file string) (interface{}, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.DeserializeFromStream(t, f)
}

// DeserializeFromBytes deserializes from a byte slice
func (s *MyJsonSerializer) DeserializeFromBytes(t reflect.Type, buffer []byte) (interface{}, error) {
	reader := bytes.NewReader(buffer)
	return s.DeserializeFromStream(t, reader)
}
