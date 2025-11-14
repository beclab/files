package serialization

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
)

// MyXmlSerializer implements IXmlSerializer for XML serialization
type MyXmlSerializer struct {
	serializers sync.Map // Cache of type to *xml.Encoder/*xml.Decoder
}

// NewMyXmlSerializer creates a new MyXmlSerializer
func NewMyXmlSerializer() *MyXmlSerializer {
	return &MyXmlSerializer{}
}

// getSerializer retrieves or creates an XML encoder/decoder for the given type
func (s *MyXmlSerializer) getSerializer(t reflect.Type) interface{} {
	key := t.String()
	if serializer, ok := s.serializers.Load(key); ok {
		return serializer
	}
	// In Go, we don't need a separate serializer instance like in C#;
	// xml.Marshal/Unmarshal are sufficient, but we cache the type for consistency
	s.serializers.Store(key, t)
	return t
}

// SerializeToWriter serializes an object to an XML writer
func (s *MyXmlSerializer) serializeToWriter(obj interface{}, writer io.Writer) error {
	encoder := xml.NewEncoder(writer)
	encoder.Indent("", "  ") // Equivalent to Formatting.Indented
	return encoder.Encode(obj)
}

// DeserializeFromStream deserializes from a stream
func (s *MyXmlSerializer) DeserializeFromStream(t reflect.Type, r io.Reader) (interface{}, error) {
	decoder := xml.NewDecoder(r)
	v := reflect.New(t).Interface()
	if err := decoder.Decode(v); err != nil {
		return nil, err
	}
	fmt.Printf("DeserializeFromStream xml: %+v\n", v)
	return v, nil
}

// SerializeToStream serializes an object to a stream
func (s *MyXmlSerializer) SerializeToStream(obj interface{}, w io.Writer) error {
	// Use a buffer to ensure proper handling
	var buf bytes.Buffer
	if err := s.serializeToWriter(obj, &buf); err != nil {
		return err
	}
	_, err := io.Copy(w, &buf)
	return err
}

// SerializeToFile serializes an object to a file
func (s *MyXmlSerializer) SerializeToFile(obj interface{}, file string) error {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.SerializeToStream(obj, f)
}

// DeserializeFromFile deserializes from a file
func (s *MyXmlSerializer) DeserializeFromFile(t reflect.Type, file string) (interface{}, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.DeserializeFromStream(t, f)
}

// DeserializeFromBytes deserializes from a byte slice
func (s *MyXmlSerializer) DeserializeFromBytes(t reflect.Type, buffer []byte) (interface{}, error) {
	reader := bytes.NewReader(buffer)
	return s.DeserializeFromStream(t, reader)
}
