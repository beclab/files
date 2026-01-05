package searpc

import (
	"net"
	"os"
	"strings"
	"testing"
)

// TestSend_Actual_EINVAL triggers real EINVAL in send()
func TestSend_Actual_EINVAL(t *testing.T) {
	invalidPath := "/invalid/socket/path"
	os.Remove(invalidPath)

	client := NewNamedPipeClient(invalidPath, "", 1)
	transport := &NamedPipeTransport{
		socketPath: invalidPath,
		client:     client,
	}

	// Call send() to trigger connection error
	_, err := transport.Send("test", "data")
	if err == nil {
		t.Fatal("Expected connection error")
	}

	// Verify error type and retry logic
	if !strings.Contains(err.Error(), "syscall.EINVAL") {
		t.Errorf("Expected EINVAL, got: %v", err)
	}
	if !isNonRetryableError(err) {
		t.Errorf("Expected non-retryable error")
	}
}

// TestSend_Actual_ENOTCONN triggers real ENOTCONN in send()
func TestSend_Actual_ENOTCONN(t *testing.T) {
	validPath := "/tmp/test_socket_ENOTCONN"
	os.Remove(validPath)

	client := NewNamedPipeClient(validPath, "", 1)
	transport := &NamedPipeTransport{
		socketPath: validPath,
		client:     client,
	}

	// Establish valid connection
	if err := transport.Connect(); err != nil {
		t.Fatal(err)
	}
	transport.Stop()

	// Call send() to trigger connection error
	_, err := transport.Send("test", "data")
	if err == nil {
		t.Fatal("Expected ENOTCONN error")
	}

	// Verify error type
	if !strings.Contains(err.Error(), "syscall.ENOTCONN") {
		t.Errorf("Expected ENOTCONN, got: %v", err)
	}
	if !isNonRetryableError(err) {
		t.Errorf("Expected non-retryable error")
	}
}

// TestSend_Actual_EADDRINUSE triggers real EADDRINUSE in send()
func TestSend_Actual_EADDRINUSE(t *testing.T) {
	validPath := "/tmp/test_socket_EADDRINUSE"
	os.Remove(validPath)

	// First listener
	listener, err := net.Listen("unix", validPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	// Create client
	client := NewNamedPipeClient(validPath, "", 1)
	transport := &NamedPipeTransport{
		socketPath: validPath,
		client:     client,
	}

	// Call send() to trigger address use error
	_, err = transport.Send("test", "data")
	if err == nil {
		t.Fatal("Expected EADDRINUSE error")
	}

	// Verify error type
	if !strings.Contains(err.Error(), "syscall.EADDRINUSE") {
		t.Errorf("Expected EADDRINUSE, got: %v", err)
	}
	if !isNonRetryableError(err) {
		t.Errorf("Expected non-retryable error")
	}
}
