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
	klog.Infof("Send called - service: %s, fcallStr: %s", service, fcallStr)

	reqBody := map[string]string{
		"service": service,
		"request": fcallStr,
	}

	jsonData := common.ToBytes(reqBody)

	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(jsonData)))

	sendData := append(header, jsonData...)

	if _, err := t.conn.Write(sendData); err != nil {
		return "", err
	}

	respHeader := make([]byte, 4)
	_, err := t.conn.Read(respHeader)
	if err != nil {
		return "", err
	}
	respSize := binary.LittleEndian.Uint32(respHeader)

	respBody := make([]byte, respSize)
	_, err = t.conn.Read(respBody)
	if err != nil {
		return "", err
	}

	respStr := string(respBody)

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
			return nil, err
		}
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
	transport, err := c.getTransport()
	if err != nil {
		return "", err
	}
	defer c.returnTransport(transport)

	return transport.Send(c.serviceName, fcallStr)
}
