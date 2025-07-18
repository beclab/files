package gosearpc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// NamedPipeException 命名管道异常类型
type NamedPipeException struct {
	msg string
}

func (e *NamedPipeException) Error() string {
	return e.msg
}

// NamedPipeTransport 命名管道传输层实现
type NamedPipeTransport struct {
	SearpcTransport
	socketPath string
	conn       net.Conn
}

// 在NewNamedPipeClient中添加连接验证
func (c *NamedPipeClient) validateTransport(t *NamedPipeTransport) bool {
	return t.conn != nil
}

// Connect 连接到命名管道服务器
func (t *NamedPipeTransport) Connect() error {
	conn, err := net.Dial("unix", t.socketPath)
	if err != nil {
		return err
	}
	t.conn = conn
	return nil
}

// Stop 关闭连接
func (t *NamedPipeTransport) Stop() {
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
}

// Send 发送RPC请求并接收响应
func (t *NamedPipeTransport) Send(service, fcallStr string) (string, error) {
	// 构造请求体
	reqBody := map[string]string{
		"service": service,
		"request": fcallStr,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// 打包长度头（4字节无符号整数）
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(jsonData)))

	// 发送数据
	if _, err := t.conn.Write(append(header, jsonData...)); err != nil {
		return "", err
	}

	// 接收响应头
	respHeader := make([]byte, 4)
	if _, err := t.conn.Read(respHeader); err != nil {
		return "", err
	}
	respSize := binary.LittleEndian.Uint32(respHeader)

	// 接收响应体
	respBody := make([]byte, respSize)
	if _, err := t.conn.Read(respBody); err != nil {
		return "", err
	}

	return string(respBody), nil
}

// NamedPipeClient 命名管道客户端实现
type NamedPipeClient struct {
	SearpcClient
	socketPath  string
	serviceName string
	poolSize    int
	pool        chan *NamedPipeTransport
	mu          sync.Mutex
}

// NewNamedPipeClient 创建命名管道客户端
func NewNamedPipeClient(socketPath, serviceName string, poolSize int) *NamedPipeClient {
	return &NamedPipeClient{
		socketPath:  socketPath,
		serviceName: serviceName,
		poolSize:    poolSize,
		pool:        make(chan *NamedPipeTransport, poolSize),
	}
}

// getTransport 从连接池获取传输实例
func (c *NamedPipeClient) getTransport() (*NamedPipeTransport, error) {
	select {
	case t := <-c.pool:
		return t, nil
	default:
		// 创建新连接
		transport := &NamedPipeTransport{socketPath: c.socketPath}
		if err := transport.Connect(); err != nil {
			return nil, err
		}
		return transport, nil
	}
}

// returnTransport 归还传输实例到连接池
func (c *NamedPipeClient) returnTransport(t *NamedPipeTransport) {
	select {
	case c.pool <- t:
	default:
		// 池已满则关闭连接
		t.Stop()
	}
}

// CallRemoteFuncSync 同步调用远程函数
func (c *NamedPipeClient) CallRemoteFuncSync(fcallStr string) (string, error) {
	transport, err := c.getTransport()
	if err != nil {
		return "", err
	}
	defer c.returnTransport(transport)

	return transport.Send(c.serviceName, fcallStr)
}

// NamedPipeServer 命名管道服务器实现
type NamedPipeServer struct {
	socketPath string
	listener   net.Listener
}

// NewNamedPipeServer 创建命名管道服务器
func NewNamedPipeServer(socketPath string) *NamedPipeServer {
	return &NamedPipeServer{socketPath: socketPath}
}

// Start 启动服务器
func (s *NamedPipeServer) Start() error {
	// 清理已存在的socket文件
	if _, err := os.Stat(s.socketPath); err == nil {
		if err := os.Remove(s.socketPath); err != nil {
			return &NamedPipeException{"Failed to remove existing socket file"}
		}
	}

	// 创建监听
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}
	s.listener = listener

	// 启动接受循环
	go s.acceptLoop()
	return nil
}

// Stop 停止服务器
func (s *NamedPipeServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
		os.Remove(s.socketPath)
	}
}

// acceptLoop 接受客户端连接
func (s *NamedPipeServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

// handleConnection 处理客户端连接
func (s *NamedPipeServer) handleConnection(conn net.Conn) {
	// 在handleConnection中添加请求超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	defer conn.Close()

	buf := make([]byte, 4096)
	for {
		// 添加超时检查
		select {
		case <-ctx.Done():
			log.Println("请求处理超时，强制关闭连接")
			return
		default:
			// 继续正常处理流程
		}

		// 读取请求头
		if _, err := conn.Read(buf[:4]); err != nil {
			log.Printf("Read header error: %v", err)
			return
		}
		reqSize := binary.LittleEndian.Uint32(buf[:4])

		// 读取请求体
		reqBody := make([]byte, reqSize)
		if _, err := conn.Read(reqBody); err != nil {
			log.Printf("Read body error: %v", err)
			return
		}

		// 解析JSON请求
		var reqData map[string]string
		if err := json.Unmarshal(reqBody, &reqData); err != nil {
			log.Printf("JSON parse error: %v", err)
			return
		}

		// 调用服务函数（此处需要实现searpc_server.call_function）
		// resp := searpc_server.call_function(reqData["service"], reqData["request"])
		//resp := "{}" // 占位符，需后续实现
		resp := GlobalSearpcServer.CallFunction(reqData["service"], reqData["request"])

		// 打包响应
		respHeader := make([]byte, 4)
		binary.LittleEndian.PutUint32(respHeader, uint32(len(resp)))
		if _, err := conn.Write(append(respHeader, []byte(resp)...)); err != nil {
			log.Printf("Write response error: %v", err)
			return
		}
	}
}

// 工具函数实现（需在transport.go中实现）
/*
func recvall(conn net.Conn, size int) ([]byte, error) {
	buf := make([]byte, size)
	_, err := io.ReadFull(conn, buf)
	return buf, err
}

func sendall(conn net.Conn, data []byte) error {
	_, err := conn.Write(data)
	return err
}
*/
