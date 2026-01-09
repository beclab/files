package searpc

import (
	"encoding/binary"
	"files/pkg/common"
	"fmt"
	"net"
	"strings"
	"sync"

	"k8s.io/klog/v2"
)

type NamedPipeException struct {
	msg string
}

func (e *NamedPipeException) Error() string {
	return e.msg
}

type NamedPipeTransport struct {
	SearpcTransport
	socketPath string
	conn       net.Conn
	client     *NamedPipeClient
}

func (c *NamedPipeClient) validateTransport(t *NamedPipeTransport) bool {
	return t.conn != nil
}

func (t *NamedPipeTransport) Connect() error {
	if t.conn == nil {
		conn, err := net.Dial("unix", t.socketPath)
		if err != nil {
			return err
		}
		t.conn = conn
	}
	return nil
}

func (t *NamedPipeTransport) Stop() {
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
}

func (t *NamedPipeTransport) Send(service, fcallStr string) (string, error) {
	klog.Infof("Send called - service: %s, fcallStr: %s", service, fcallStr)
	var respStr string
	var err error

	if respStr, err = t.trySend(service, fcallStr); err == nil {
		return respStr, nil
	}

	if isRetryableError(err) {
		klog.Errorf("[RPC] sending has met retryable err: %v, retry once immediately", err)
		if respStr, err = t.trySend(service, fcallStr); err == nil {
			return respStr, nil
		}
	}
	klog.Errorf("[RPC] send error, err: %v", err)
	return "", fmt.Errorf("sync server connection failed")
}

func (t *NamedPipeTransport) trySend(service, fcallStr string) (string, error) {
	if err := t.Connect(); err != nil {
		return "", err
	}

	reqBody := map[string]string{
		"service": service,
		"request": fcallStr,
	}
	jsonData := common.ToBytes(reqBody)
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(jsonData)))
	sendData := append(header, jsonData...)

	if _, err := t.conn.Write(sendData); err != nil {
		t.handleConnectionError(err)
		return "", err
	}

	respHeader := make([]byte, 4)
	if _, err := t.conn.Read(respHeader); err != nil {
		t.handleConnectionError(err)
		return "", err
	}
	respSize := binary.LittleEndian.Uint32(respHeader)
	respBody := make([]byte, respSize)
	if _, err := t.conn.Read(respBody); err != nil {
		t.handleConnectionError(err)
		return "", err
	}
	return string(respBody), nil
}

func (t *NamedPipeTransport) handleConnectionError(connErr error) {
	klog.Errorf("[RPC] Connection Error: %v", connErr)
	t.Stop()
	_, err := t.client.syncTransport(t)
	if err != nil {
		klog.Errorf("Failed to refresh transport: %v", err)
	}
}

func isRetryableError(err error) bool {
	retryable := []string{
		"broken pipe",
	}

	errMsg := err.Error()
	for _, e := range retryable {
		if strings.Contains(errMsg, e) {
			return true
		}
	}
	return false
}

func SyncConnectionFailedError(err error) error {
	// only call this func for judging at given special scenes
	syncConnectionFails := []string{
		"broken pipe",        // for common rpc calling
		"connection refused", // for syncing transport
		"connection reset",   // for uploading to sync
		"Bad Gateway",        // for copying to sync
		"unexpected EOF",     // for copying from sync
	}

	errMsg := err.Error()
	for _, e := range syncConnectionFails {
		if strings.Contains(errMsg, e) {
			return fmt.Errorf("sync server connection failed")
		}
	}
	return err
}

type NamedPipeClient struct {
	SearpcClient
	socketPath  string
	serviceName string
	poolSize    int
	pool        chan *NamedPipeTransport
	mu          sync.Mutex
}

func NewNamedPipeClient(socketPath, serviceName string, poolSize int) *NamedPipeClient {
	return &NamedPipeClient{
		socketPath:  socketPath,
		serviceName: serviceName,
		poolSize:    poolSize,
		pool:        make(chan *NamedPipeTransport, poolSize),
	}
}

func (c *NamedPipeClient) getTransport() (*NamedPipeTransport, error) {
	select {
	case t := <-c.pool:
		if t.conn != nil {
			return t, nil
		}
		return c.syncTransport(nil)
	default:
		return c.syncTransport(nil)
	}
}

func (c *NamedPipeClient) syncTransport(old *NamedPipeTransport) (*NamedPipeTransport, error) {
	klog.Infof("[RPC] Creating New Transport")
	newT := &NamedPipeTransport{
		socketPath: c.socketPath,
		client:     c,
	}
	if err := newT.Connect(); err != nil {
		klog.Errorf("[RPC] Failed to connect: %v", err)
		return nil, fmt.Errorf("sync server connection failed")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if old != nil {
		klog.Infof("[RPC] Stop Old Transport")
		old.Stop()
	}

	select {
	case c.pool <- newT:
		klog.Infof("[RPC] Transport Added to Pool")
		return newT, nil
	default:
		newT.Stop()
		klog.Infof("[RPC] Transport Rejected from Pool")
		return nil, fmt.Errorf("connection pool full")
	}
}

func (c *NamedPipeClient) returnTransport(t *NamedPipeTransport) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if t.conn != nil {
		select {
		case c.pool <- t:
		default:
			t.Stop()
		}
	}
}

func (c *NamedPipeClient) CallRemoteFuncSync(fcallStr string) (string, error) {
	transport, err := c.getTransport()
	if err != nil {
		return "", err
	}
	defer c.returnTransport(transport)
	return transport.Send(c.serviceName, fcallStr)
}
