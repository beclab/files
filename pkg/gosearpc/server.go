// 文件名：server.go
package gosearpc

import (
	"encoding/json"
	"fmt"
)

// SearpcService 定义RPC服务结构
type SearpcService struct {
	Name      string                 // 服务名称
	FuncTable map[string]interface{} // 函数表，存储注册的函数
}

// SearpcServer 定义RPC服务器结构
type SearpcServer struct {
	Services map[string]*SearpcService // 注册的服务集合
}

// 错误响应结构体
type errorResponse struct {
	ErrCode int    `json:"err_code"`
	ErrMsg  string `json:"err_msg"`
}

// 正常响应结构体
type successResponse struct {
	Ret interface{} `json:"ret"`
}

// NewSearpcServer 创建新的RPC服务器实例
func NewSearpcServer() *SearpcServer {
	return &SearpcServer{
		Services: make(map[string]*SearpcService),
	}
}

// CreateService 创建新的服务实例
func (s *SearpcServer) CreateService(svcname string) {
	s.Services[svcname] = &SearpcService{
		Name:      svcname,
		FuncTable: make(map[string]interface{}),
	}
}

// RegisterFunction 注册函数到指定服务
func (s *SearpcServer) RegisterFunction(svcname string, fn interface{}, fname string) {
	service, exists := s.Services[svcname]
	if !exists {
		panic(fmt.Sprintf("Service %s not exists", svcname))
	}

	if fname == "" {
		// 此处假设后续实现中会处理函数名称提取
		fname = "anonymous_function"
	}
	service.FuncTable[fname] = fn
}

// callFunction 执行函数调用（内部方法）
func (s *SearpcServer) callFunction(svcname string, fcallstr string) (interface{}, error) {
	// 解析JSON参数
	var argv []interface{}
	if err := json.Unmarshal([]byte(fcallstr), &argv); err != nil {
		return nil, &SearpcError{Msg: "bad call str: " + err.Error()}
	}

	// 获取服务实例
	service, exists := s.Services[svcname]
	if !exists {
		return nil, &SearpcError{Msg: "Service not exists"}
	}

	// 获取函数引用
	if len(argv) == 0 {
		return nil, &SearpcError{Msg: "Empty function name"}
	}
	fname, ok := argv[0].(string)
	if !ok {
		return nil, &SearpcError{Msg: "Invalid function name type"}
	}

	fn, exists := service.FuncTable[fname]
	if !exists {
		return nil, &SearpcError{Msg: "No such function " + fname}
	}

	// 执行函数调用（假设函数参数类型匹配）
	// 注意：此处需要后续实现参数类型转换逻辑
	return fn, nil
}

// CallFunction 处理RPC调用请求
func (s *SearpcServer) CallFunction(svcname string, fcallstr string) string {
	defer func() {
		if r := recover(); r != nil {
			// 捕获panic并转换为错误响应
			errMsg := fmt.Sprintf("Internal error: %v", r)
			resp, _ := json.Marshal(errorResponse{
				ErrCode: 555,
				ErrMsg:  errMsg,
			})
			fmt.Println(string(resp))
		}
	}()

	// 执行函数调用
	retVal, err := s.callFunction(svcname, fcallstr)
	if err != nil {
		// 错误响应
		resp, _ := json.Marshal(errorResponse{
			ErrCode: 555,
			ErrMsg:  err.Error(),
		})
		return string(resp)
	}

	// 正常响应
	resp, _ := json.Marshal(successResponse{
		Ret: retVal,
	})
	return string(resp)
}

// 全局服务器实例
var GlobalSearpcServer = NewSearpcServer()
