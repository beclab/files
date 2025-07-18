package goseafile

import (
	"encoding/json"
	"files/pkg/gosearpc"
	"fmt"
	"k8s.io/klog/v2"
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
	klog.Infof("~~~Debug log: Creating RPC method %s with return type %s and %d parameters", methodName, retType, len(paramTypes))

	decorator := searpcFunc(methodName, retType, paramTypes)

	return func(args ...interface{}) (interface{}, error) {
		klog.Infof("~~~Debug log: Starting RPC call to %s with %d arguments", methodName, len(args))

		// 参数校验
		if len(args) != len(paramTypes) {
			klog.Infof("~~~Debug log: Parameter mismatch for %s - expected %d, got %d", methodName, len(paramTypes), len(args))
			return nil, fmt.Errorf(
				"参数数量不匹配: 方法 %s 需要 %d 个参数，实际收到 %d 个",
				methodName,
				len(paramTypes),
				len(args),
			)
		}

		// 记录参数详情
		for i, arg := range args {
			klog.Infof("~~~Debug log: Parameter %d (type %T): %+v", i, arg, arg)
		}

		// 构造完整参数列表
		callArgs := append([]interface{}{methodName}, args...)
		klog.Infof("~~~Debug log: Full argument list for %s: %+v", methodName, callArgs)

		// 序列化参数
		data, err := json.Marshal(callArgs)
		if err != nil {
			klog.Infof("~~~Debug log: Failed to marshal arguments for %s: %v", methodName, err)
			return nil, fmt.Errorf("参数序列化失败: %v", err)
		}
		klog.Infof("~~~Debug log: Serialized arguments for %s: %s", methodName, string(data))

		var result interface{}
		var rpcErr error

		decoratedFunc := decorator(func(sc SearpcClient, _ ...interface{}) (string, error) {
			klog.Infof("~~~Debug log: Calling remote function %s via named pipe", methodName)
			resp, err := sc.CallRemoteFuncSync(string(data))
			klog.Infof("~~~Debug log: Remote call response for %s: %s, error: %v", methodName, resp, err)
			return resp, err
		})

		// 执行调用
		result, rpcErr = decoratedFunc(c.NamedPipeClient)
		if rpcErr != nil {
			klog.Infof("~~~Debug log: RPC call failed for %s: %v", methodName, rpcErr)
			return nil, fmt.Errorf("RPC调用失败: %v", rpcErr)
		}

		// 记录原始响应
		klog.Infof("~~~Debug log: Raw response for %s: %+v (type: %T)", methodName, result, result)

		// 增强类型转换处理
		switch retType {
		case "void": // 新增 void 类型处理
			klog.Infof("~~~Debug log: Void result for %s (no return value expected)", methodName)
			return nil, nil // 显式返回 nil
		case "int":
			return handleIntConversion(result, methodName)
		case "string":
			return handleStringConversion(result, methodName)
		case "objlist":
			return handleObjListConversion(result, methodName)
		case "object":
			return handleObjectConversion(result, methodName)
		case "json":
			return handleJsonConversion(result, methodName)
		default:
			klog.Infof("~~~Debug log: Returning raw result for %s (type: %T): %+v",
				methodName, result, result)
			return result, nil
		}
	}
}

// 新增类型转换处理函数
func handleIntConversion(result interface{}, methodName string) (int, error) {
	if s, ok := result.(string); ok {
		val, err := strconv.Atoi(s)
		if err != nil {
			klog.Infof("~~~Debug log: Failed to convert int result for %s: %v", methodName, err)
			return 0, fmt.Errorf("类型转换失败: %v", err)
		}
		klog.Infof("~~~Debug log: Successfully converted int result for %s: %d", methodName, val)
		return val, nil
	}
	return 0, fmt.Errorf("返回类型错误: 期望string，实际得到%T", result)
}

func handleStringConversion(result interface{}, methodName string) (string, error) {
	if s, ok := result.(string); ok {
		klog.Infof("~~~Debug log: String result for %s: %s", methodName, s)
		return s, nil
	}
	return "", fmt.Errorf("返回类型错误: 期望string，实际得到%T", result)
}

func handleObjListConversion(result interface{}, methodName string) ([]*gosearpc.SearpcObj, error) {
	if list, ok := result.([]*gosearpc.SearpcObj); ok {
		klog.Infof("~~~Debug log: Successfully parsed objlist for %s, count: %d",
			methodName, len(list))
		return list, nil
	}
	return nil, fmt.Errorf("返回类型错误: 期望[]*_SearpcObj，实际得到%T", result)
}

func handleObjectConversion(result interface{}, methodName string) (*gosearpc.SearpcObj, error) {
	if obj, ok := result.(*gosearpc.SearpcObj); ok {
		klog.Infof("~~~Debug log: Successfully parsed object for %s: %+v", methodName, obj)
		return obj, nil
	}
	return nil, fmt.Errorf("返回类型错误: 期望*_SearpcObj，实际得到%T", result)
}

func handleJsonConversion(result interface{}, methodName string) (interface{}, error) {
	// 根据实际需要实现JSON解析逻辑
	klog.Infof("~~~Debug log: JSON result for %s: %+v", methodName, result)
	return result, nil
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

func (c *SeafServerThreadedRpcClient) CountEmailusers(source string) (interface{}, error) {
	return c.createRPCMethod("count_emailusers", "int64", []string{"string"})(
		source)
}

func (c *SeafServerThreadedRpcClient) CountInactiveEmailusers(source string) (interface{}, error) {
	return c.createRPCMethod("count_inactive_emailusers", "int64", []string{"string"})(
		source)
}
