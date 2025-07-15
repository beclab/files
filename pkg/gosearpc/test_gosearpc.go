// 包名：gosearpc
package gosearpc

import (
	"encoding/json"
	"log"
	"os"
	"testing"
)

// 定义服务名称常量
const SVCNAME = "test-service"

// 初始化服务（假设后续文件已实现）
func initServer() {
	// 以下函数假设已在其他文件实现
	GlobalSearpcServer.CreateService(SVCNAME)
	GlobalSearpcServer.RegisterFunction(SVCNAME, add, "add")
	GlobalSearpcServer.RegisterFunction(SVCNAME, mul, "multi")
	GlobalSearpcServer.RegisterFunction(SVCNAME, jsonFunc, "json_func")
	GlobalSearpcServer.RegisterFunction(SVCNAME, getStr, "get_str")
}

// 测试用函数定义
func add(x, y int) int { return x + y }
func mul(x string, y int) string {
	result := ""
	for i := 0; i < y; i++ {
		result += x
	}
	return result
}
func jsonFunc(a, b int) map[string]interface{} {
	return map[string]interface{}{"a": a, "b": b}
}
func getStr() string { return "这是一个测试" }

// DummyTransport 实现（模拟传输层）
type DummyTransport struct {
	SearpcTransport
}

func (t *DummyTransport) Connect() {}
func (t *DummyTransport) Send(service string, fcallStr string) string {
	return GlobalSearpcServer.CallFunction(service, fcallStr)
}

// RpcMixin 定义客户端方法
type RpcMixin struct{}

//go:noinline
func (m *RpcMixin) Add(x, y int) (int, error) { return 0, nil }

//go:noinline
func (m *RpcMixin) Multi(x string, y int) (string, error) { return "", nil }

//go:noinline
func (m *RpcMixin) JsonFunc(x, y int) (map[string]interface{}, error) { return nil, nil }

//go:noinline
func (m *RpcMixin) GetStr() (string, error) { return "", nil }

// DummyRpcClient 实现
type DummyRpcClient struct {
	RpcMixin
	transport *DummyTransport
}

func NewDummyRpcClient() *DummyRpcClient {
	return &DummyRpcClient{
		transport: &DummyTransport{},
	}
}

func (c *DummyRpcClient) CallRemoteFuncSync(fcallStr string) string {
	return c.transport.Send(SVCNAME, fcallStr)
}

// NamedPipeClientForTest 实现
type NamedPipeClientForTest struct {
	RpcMixin
	// 假设已实现命名管道客户端
}

// 测试套件
var (
	client          *DummyRpcClient
	namedPipeServer *NamedPipeServer //SearpcServer
	namedPipeClient *NamedPipeClientForTest
)

// TestMain 测试入口
func TestMain(m *testing.M) {
	setupLogging()
	initServer()
	client = NewDummyRpcClient()

	// 启动命名管道服务端（假设已实现）
	namedPipeServer = NewNamedPipeServer(SOCKET_PATH)
	namedPipeServer.Start()
	namedPipeClient = &NamedPipeClientForTest{}

	code := m.Run()
	namedPipeServer.Stop()
	os.Exit(code)
}

// 测试普通传输
func TestNormalTransport(t *testing.T) {
	runCommonTestsDummy(t, client)
}

func runCommonTestsDummy(t *testing.T, client *DummyRpcClient) {
	t.Run("Add", func(t *testing.T) {
		// 假设已实现远程调用方法
		result, _ := client.Add(1, 2)
		if result != 3 {
			t.Errorf("Expected 3, got %d", result)
		}
	})

	t.Run("Multi", func(t *testing.T) {
		result, _ := client.Multi("abc", 2)
		if result != "abcabc" {
			t.Errorf("Expected abcabc, got %s", result)
		}
	})

	t.Run("JsonFunc", func(t *testing.T) {
		result, _ := client.JsonFunc(1, 2)
		expected := map[string]interface{}{"a": 1.0, "b": 2.0}
		if !deepEqual(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("GetStr", func(t *testing.T) {
		result, _ := client.GetStr()
		if result != "这是一个测试" {
			t.Errorf("Expected 这是一个测试, got %s", result)
		}
	})
}

// 测试管道传输
func TestPipeTransport(t *testing.T) {
	runCommonTestsNamed(t, namedPipeClient)
}

// 通用测试逻辑
func runCommonTestsNamed(t *testing.T, client *NamedPipeClientForTest) {
	t.Run("Add", func(t *testing.T) {
		// 假设已实现远程调用方法
		result, _ := client.Add(1, 2)
		if result != 3 {
			t.Errorf("Expected 3, got %d", result)
		}
	})

	t.Run("Multi", func(t *testing.T) {
		result, _ := client.Multi("abc", 2)
		if result != "abcabc" {
			t.Errorf("Expected abcabc, got %s", result)
		}
	})

	t.Run("JsonFunc", func(t *testing.T) {
		result, _ := client.JsonFunc(1, 2)
		expected := map[string]interface{}{"a": 1.0, "b": 2.0}
		if !deepEqual(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("GetStr", func(t *testing.T) {
		result, _ := client.GetStr()
		if result != "这是一个测试" {
			t.Errorf("Expected 这是一个测试, got %s", result)
		}
	})
}

// 辅助函数：深度比较JSON结果
func deepEqual(a, b map[string]interface{}) bool {
	ajson, _ := json.Marshal(a)
	bjson, _ := json.Marshal(b)
	return string(ajson) == string(bjson)
}

// 日志设置
func setupLogging() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)
}

const SOCKET_PATH = "/tmp/libsearpc-test.sock"
