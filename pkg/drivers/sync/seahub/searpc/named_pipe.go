package searpc

import (
	"encoding/binary"
	"files/pkg/common"
	"k8s.io/klog/v2"
	"net"
	"sync"
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
}

func (c *NamedPipeClient) validateTransport(t *NamedPipeTransport) bool {
	return t.conn != nil
}

func (t *NamedPipeTransport) Connect() error {
	conn, err := net.Dial("unix", t.socketPath)
	if err != nil {
		return err
	}
	t.conn = conn
	return nil
}

func (t *NamedPipeTransport) Stop() {
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
}

func (t *NamedPipeTransport) Send(service, fcallStr string) (string, error) {
	klog.Infof("~~~Debug log: Send called - service: %s, fcallStr: %s", service, fcallStr)

	reqBody := map[string]string{
		"service": service,
		"request": fcallStr,
	}

	jsonData := common.ToBytes(reqBody)
	klog.Infof("~~~Debug log: Request JSON - service: %s, json: %s", service, string(jsonData))

	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(jsonData)))
	klog.Infof("~~~Debug log: Header created - service: %s, header: %x, data length: %d",
		service, header, len(jsonData))

	sendData := append(header, jsonData...)
	klog.Infof("~~~Debug log: Sending data - service: %s, total length: %d, data: %x",
		service, len(sendData), sendData)

	if _, err := t.conn.Write(sendData); err != nil {
		klog.Errorf("~~~Debug log: Write failed - service: %s, error: %v", service, err)
		return "", err
	}

	respHeader := make([]byte, 4)
	n, err := t.conn.Read(respHeader)
	if err != nil {
		klog.Errorf("~~~Debug log: Read header failed - service: %s, read bytes: %d, error: %v",
			service, n, err)
		return "", err
	}
	respSize := binary.LittleEndian.Uint32(respHeader)
	klog.Infof("~~~Debug log: Received header - service: %s, header: %x, respSize: %d",
		service, respHeader, respSize)

	respBody := make([]byte, respSize)
	n, err = t.conn.Read(respBody)
	if err != nil {
		klog.Errorf("~~~Debug log: Read body failed - service: %s, expected: %d, read: %d, error: %v",
			service, respSize, n, err)
		return "", err
	}

	respStr := string(respBody)
	klog.Infof("~~~Debug log: Received response - service: %s, length: %d, content: %s",
		service, len(respStr), respStr)

	return respStr, nil
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
		return t, nil
	default:
		transport := &NamedPipeTransport{socketPath: c.socketPath}
		if err := transport.Connect(); err != nil {
			klog.Errorf("~~~Debug log: Failed to connect to socket: %v", err)
			return nil, err
		}
		klog.Infof("~~~Debug log: Success to connect to socket: %v", transport)
		return transport, nil
	}
}

func (c *NamedPipeClient) returnTransport(t *NamedPipeTransport) {
	select {
	case c.pool <- t:
	default:
		t.Stop()
	}
}

func (c *NamedPipeClient) CallRemoteFuncSync(fcallStr string) (string, error) {
	klog.Infof("~~~Debug log: Call remote function sync - fcallStr: %s", fcallStr)
	transport, err := c.getTransport()
	if err != nil {
		klog.Errorf("~~~Debug log: Failed to connect to socket: %v", err)
		return "", err
	}
	defer c.returnTransport(transport)

	return transport.Send(c.serviceName, fcallStr)
}
