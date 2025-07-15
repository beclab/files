// 文件名：transport.go
package gosearpc

type SearpcTransportInterface interface {
	Connect()
	Send(serviceName string, requestStr string) string
}

// SearpcTransport 定义RPC传输层的抽象接口
// 负责序列化请求的发送和原始响应的接收
type SearpcTransport struct{}

// Connect 建立到服务器的物理连接
// 具体实现需要由具体传输类型（如NamedPipeTransport）完成
// 示例实现应包含实际的连接建立逻辑
func (t *SearpcTransport) Connect() {
	panic("Connect method must be implemented by concrete transport type")
}

// Send 执行完整的请求-响应周期
// 参数:
//
//	serviceName - 目标服务名称
//	requestStr - 序列化后的请求字符串
//
// 返回:
//
//	string - 服务器返回的原始响应数据
//
// 注意：具体实现需要处理网络通信细节
func (t *SearpcTransport) Send(serviceName string, requestStr string) string {
	panic("Send method must be implemented by concrete transport type")
}
