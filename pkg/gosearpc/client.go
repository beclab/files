package gosearpc

import (
	"encoding/json"
	"k8s.io/klog/v2"
	"strings"
)

// SearpcClient 定义客户端接口
type SearpcClientInterface interface {
	CallRemoteFuncSync(fcallStr string) (string, error)
}

type SearpcClient struct{}

func (c *SearpcClient) CallRemoteFuncSync(fcallStr string) (string, error) {
	// 这里添加实际的远程调用实现
	return "", nil
}

// _fret_int 解析整型返回值
func _fret_int(retStr string) (int, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return 0, &SearpcError{"Invalid response format"}
	}

	if errCode, ok := dicts["err_code"]; ok {
		klog.Infof("~~~Debug log: errCode: %s", errCode.(string))
		return 0, &SearpcError{dicts["err_msg"].(string)}
	}

	if ret, ok := dicts["ret"]; ok {
		if f, ok := ret.(float64); ok {
			return int(f), nil
		}
		return 0, &SearpcError{"Invalid response format"}
	}
	return 0, &SearpcError{"Invalid response format"}
}

// _fret_string 解析字符串返回值
func _fret_string(retStr string) (string, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return "", &SearpcError{"Invalid response format"}
	}

	if errCode, ok := dicts["err_code"]; ok {
		klog.Infof("~~~Debug log: errCode: %s", errCode.(string))
		return "", &SearpcError{dicts["err_msg"].(string)}
	}

	if ret, ok := dicts["ret"]; ok {
		if s, ok := ret.(string); ok {
			return s, nil
		}
		return "", &SearpcError{"Invalid response format"}
	}
	return "", &SearpcError{"Invalid response format"}
}

// _SearpcObj 对象封装
type _SearpcObj struct {
	props *_SearpcObj
	data  map[string]interface{}
}

func NewSearpcObj(dict map[string]interface{}) *_SearpcObj {
	newData := make(map[string]interface{})
	for k, v := range dict {
		newKey := strings.ReplaceAll(k, "-", "_")
		newData[newKey] = v
	}
	return &_SearpcObj{
		props: &_SearpcObj{data: newData},
		data:  newData,
	}
}

func (o *_SearpcObj) Get(key string) interface{} {
	if val, ok := o.data[key]; ok {
		return val
	}
	return nil
}

func (o *_SearpcObj) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.data)
}

// _fret_obj 解析对象
func _fret_obj(retStr string) (*_SearpcObj, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return nil, &SearpcError{"Invalid response format"}
	}

	if errCode, ok := dicts["err_code"]; ok {
		klog.Infof("~~~Debug log: errCode: %s", errCode.(string))
		return nil, &SearpcError{dicts["err_msg"].(string)}
	}

	if ret, ok := dicts["ret"].(map[string]interface{}); ok {
		return NewSearpcObj(ret), nil
	}
	return nil, nil
}

// _fret_objlist 解析对象列表
func _fret_objlist(retStr string) ([]*_SearpcObj, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return nil, &SearpcError{"Invalid response format"}
	}

	if errCode, ok := dicts["err_code"]; ok {
		klog.Infof("~~~Debug log: errCode: %s", errCode.(string))
		return nil, &SearpcError{dicts["err_msg"].(string)}
	}

	var list []*_SearpcObj
	if retList, ok := dicts["ret"].([]interface{}); ok {
		for _, item := range retList {
			if dict, ok := item.(map[string]interface{}); ok {
				list = append(list, NewSearpcObj(dict))
			}
		}
	}
	return list, nil
}

// _fret_json 解析原始JSON
func _fret_json(retStr string) (interface{}, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return nil, &SearpcError{"Invalid response format"}
	}

	if errCode, ok := dicts["err_code"]; ok {
		klog.Infof("~~~Debug log: errCode: %s", errCode.(string))
		return nil, &SearpcError{dicts["err_msg"].(string)}
	}

	return dicts["ret"], nil
}

// SearpcFunc 定义RPC函数类型
type SearpcFunc func(...interface{}) (interface{}, error)

// SearpcFuncDeco 创建装饰器函数
func SearpcFuncDeco(methodName string, retType string, paramTypes []string) func(func(SearpcClient, ...interface{}) (string, error)) SearpcFunc {
	return func(f func(SearpcClient, ...interface{}) (string, error)) SearpcFunc {
		var fret func(string) (interface{}, error)

		switch retType {
		case "void":
			fret = nil
		case "object":
			fret = func(s string) (interface{}, error) { return _fret_obj(s) }
		case "objlist":
			fret = func(s string) (interface{}, error) { return _fret_objlist(s) }
		case "int", "int64":
			fret = func(s string) (interface{}, error) { return _fret_int(s) }
		case "string":
			fret = func(s string) (interface{}, error) { return _fret_string(s) }
		case "json":
			fret = func(s string) (interface{}, error) { return _fret_json(s) }
		default:
			panic(&SearpcError{"Invalid return type"})
		}

		return func(args ...interface{}) (interface{}, error) {
			funcName := methodName
			callArgs := append([]interface{}{funcName}, args...)
			klog.Infof("~~~Debug log: callArgs: %+v", callArgs)

			fcallBytes, err := json.Marshal(callArgs)
			if err != nil {
				return nil, err
			}

			// 通过接口调用
			klog.Infof("~~~Debug log: npCLient: %+v", args[0])
			npclient, ok := args[0].(*NamedPipeClient)
			if !ok {
				klog.Infof("~~~Debug log: client is not NamedPipeClient")
				//return nil, &SearpcError{"Invalid client type"}
			}
			klog.Infof("~~~Debug log: npclient: %+v", npclient)
			client, ok := args[0].(*SearpcClient)
			if !ok {
				return nil, &SearpcError{"Invalid client type"}
			}

			retStr, err := client.CallRemoteFuncSync(string(fcallBytes))
			if err != nil {
				return nil, err
			}

			if fret != nil {
				return fret(retStr)
			}
			return nil, nil
		}
	}
}
