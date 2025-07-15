package goseafile

import (
	"encoding/json"
	"files/pkg/gosearpc"
	"fmt"
	"strconv"
)

// 类型别名保持与Python一致的命名
type NamedPipeClient = gosearpc.NamedPipeClient
type SearpcClient = gosearpc.SearpcClient

var searpcFunc = gosearpc.SearpcFuncDeco

// 客户端实现（组合模式代替继承）
type SeafServerThreadedRpcClient struct {
	*NamedPipeClient
}

func NewSeafServerClient(pipePath string) *SeafServerThreadedRpcClient {
	return &SeafServerThreadedRpcClient{
		NamedPipeClient: gosearpc.NewNamedPipeClient(pipePath, "seafserv-threaded-rpcserver", 5),
	}
}

func (c *SeafServerThreadedRpcClient) createRPCMethod(
	methodName string,
	retType string,
	paramTypes []string,
) func(...interface{}) (interface{}, error) {
	// 获取方法装饰器（假设 searpcFunc 返回函数签名：func(func(SearpcClient) (string, error)) func(SearpcClient) (interface{}, error)）
	decorator := searpcFunc(retType, paramTypes)

	return func(args ...interface{}) (interface{}, error) {
		// 参数校验
		if len(args) != len(paramTypes) {
			return nil, fmt.Errorf(
				"参数数量不匹配: 方法 %s 需要 %d 个参数，实际收到 %d 个",
				methodName,
				len(paramTypes),
				len(args),
			)
		}

		// 构造完整参数列表
		callArgs := append([]interface{}{methodName}, args...)

		// 序列化参数
		data, err := json.Marshal(callArgs)
		if err != nil {
			return nil, fmt.Errorf("参数序列化失败: %v", err)
		}

		// 应用装饰器并绑定客户端实例
		var result interface{}
		var rpcErr error

		// 装饰器应该返回 func(SearpcClient) (interface{}, error)
		decoratedFunc := decorator(func(sc SearpcClient, _ ...interface{}) (string, error) {
			// 确保 data 已在外部定义并序列化
			return sc.CallRemoteFuncSync(string(data))
		})

		// 执行调用
		result, rpcErr = decoratedFunc(c.NamedPipeClient)
		if rpcErr != nil {
			return nil, fmt.Errorf("RPC调用失败: %v", rpcErr)
		}

		// 安全类型转换
		switch retType {
		case "int":
			if s, ok := result.(string); ok {
				val, err := strconv.Atoi(s)
				if err != nil {
					return nil, fmt.Errorf("类型转换失败: %v", err)
				}
				return val, nil
			}
			return nil, fmt.Errorf("返回类型错误: 期望string，实际得到%T", result)
		case "string":
			if s, ok := result.(string); ok {
				return s, nil
			}
			return nil, fmt.Errorf("返回类型错误: 期望string，实际得到%T", result)
		default:
			return result, nil
		}
	}
}

// 仓库操作接口（保持与Python完全一致的命名）
func (c *SeafServerThreadedRpcClient) SeafileCreateRepo(name, desc, ownerEmail, passwd string, encVersion int) (interface{}, error) {
	return c.createRPCMethod("seafile_create_repo", "string", []string{"string", "string", "string", "string", "int"})(
		name, desc, ownerEmail, passwd, encVersion)
}

func (c *SeafServerThreadedRpcClient) SeafileCreateEncRepo(repoID, name, desc, ownerEmail, magic, key, salt string, encVersion int) (interface{}, error) {
	return c.createRPCMethod("seafile_create_enc_repo", "string", []string{"string", "string", "string", "string", "string", "string", "string", "int"})(
		repoID, name, desc, ownerEmail, magic, key, salt, encVersion)
}

func (c *SeafServerThreadedRpcClient) SeafileGetReposByIdPrefix(idPrefix string, start, limit int) (interface{}, error) {
	return c.createRPCMethod("seafile_get_repos_by_id_prefix", "objlist", []string{"string", "int", "int"})(
		idPrefix, start, limit)
}

func (c *SeafServerThreadedRpcClient) SeafileGetRepo(repoID string) (interface{}, error) {
	return c.createRPCMethod("seafile_get_repo", "object", []string{"string"})(repoID)
}

func (c *SeafServerThreadedRpcClient) SeafileDestroyRepo(repoID string) (interface{}, error) {
	return c.createRPCMethod("seafile_destroy_repo", "int", []string{"string"})(repoID)
}

func (c *SeafServerThreadedRpcClient) GetEmailusers(source string, start int, limit int, status string) (interface{}, error) {
	return c.createRPCMethod("get_emailusers", "objlist", []string{"string", "int", "int", "string"})(
		source, start, limit, status)
}
