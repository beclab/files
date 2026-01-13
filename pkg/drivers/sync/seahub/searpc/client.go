package searpc

import (
	"encoding/json"
	"files/pkg/common"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

type SearpcClientInterface interface {
	CallRemoteFuncSync(fcallStr string) (string, error)
}

type SearpcClient struct{}

func (c *SearpcClient) CallRemoteFuncSync(fcallStr string) (string, error) {
	return "", nil
}

func _fret_int(retStr string) (int, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return 0, &SearpcError{"Invalid response format"}
	}

	if errCode, ok := dicts["err_code"]; ok {
		var errMsg string
		switch v := errCode.(type) {
		case float64:
			errMsg = fmt.Sprintf("Server error code: %d", int(v))
		case string:
			errMsg = fmt.Sprintf("Server error: %s", v)
		default:
			errMsg = "Unknown error code type"
		}

		klog.Infof("RPC Error - Code: %v(%T), Message: %q",
			errCode, errCode, dicts["err_msg"])

		if msg, ok := dicts["err_msg"].(string); ok {
			return 0, &SearpcError{msg}
		}
		return 0, &SearpcError{errMsg}
	}

	if ret, ok := dicts["ret"]; ok {
		if f, ok := ret.(float64); ok {
			return int(f), nil
		}
		return 0, &SearpcError{"Invalid response format"}
	}
	return 0, &SearpcError{"Invalid response format"}
}

func _fret_string(retStr string) (string, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return "", &SearpcError{fmt.Sprintf("Invalid JSON format: %v, raw: %s", err, retStr)}
	}

	if errCode, ok := dicts["err_code"]; ok {
		var errMsg string
		switch v := errCode.(type) {
		case float64:
			errMsg = fmt.Sprintf("Server error code: %d", int(v))
		case string:
			errMsg = fmt.Sprintf("Server error: %s", v)
		default:
			errMsg = "Unknown error code type"
		}

		klog.Infof("RPC Error - Code: %v(%T), Message: %q",
			errCode, errCode, dicts["err_msg"])

		if msg, ok := dicts["err_msg"].(string); ok {
			return "", &SearpcError{msg}
		}
		return "", &SearpcError{errMsg}
	}

	if ret, ok := dicts["ret"]; ok {
		if ret == nil {
			return "", nil
		}
		switch v := ret.(type) {
		case string:
			return v, nil
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		default:
			klog.Infof("Unexpected return type: %T, value: %v", v, v)
			return "", &SearpcError{
				fmt.Sprintf("Invalid return type: %T, need string", v)}
		}
	}

	return "", &SearpcError{
		fmt.Sprintf("Missing 'ret' field in response: %s", retStr)}
}

type SearpcObj struct {
	Props *SearpcObj
	Data  map[string]interface{}
}

func NewSearpcObj(dict map[string]interface{}) *SearpcObj {
	newData := make(map[string]interface{})
	for k, v := range dict {
		newKey := strings.ReplaceAll(k, "-", "_")
		newData[newKey] = v
	}
	ret := &SearpcObj{
		Props: nil,
		Data:  newData,
	}
	ret.Props = ret
	return ret
}

func (o *SearpcObj) Get(key string) interface{} {
	if val, ok := o.Data[key]; ok {
		return val
	}
	return nil
}

func (o *SearpcObj) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.Data)
}

func (o *SearpcObj) MapString() (map[string]string, error) {
	rawData := o.Data
	if rawData == nil {
		return nil, fmt.Errorf("empty data field")
	}

	stringMap := make(map[string]string)
	for k, v := range rawData {
		switch val := v.(type) {
		case string:
			stringMap[k] = val
		case float64:
			stringMap[k] = strconv.FormatFloat(val, 'f', -1, 64)
		case bool:
			stringMap[k] = strconv.FormatBool(val)
		case nil:
			stringMap[k] = ""
		default:
			stringMap[k] = fmt.Sprintf("%v", val)
		}
	}
	return stringMap, nil
}

func ObjListMapString(objs []*SearpcObj) ([]map[string]string, error) {
	stringMaps := make([]map[string]string, len(objs))
	errMsg := ""
	var err error
	for i, obj := range objs {
		var stringMap map[string]string
		stringMap, err = obj.MapString()
		if err != nil {
			errMsg += fmt.Sprintf("Processing object failed at index %d. ", i)
		}
		stringMaps[i] = stringMap
	}
	if errMsg != "" {
		err = fmt.Errorf("%s", errMsg)
	} else {
		err = nil
	}
	return stringMaps, err
}

func _fret_obj(retStr string) (*SearpcObj, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return nil, &SearpcError{"Invalid response format"}
	}

	if errCode, ok := dicts["err_code"]; ok {
		var errMsg string
		switch v := errCode.(type) {
		case float64:
			errMsg = fmt.Sprintf("Server error code: %d", int(v))
		case string:
			errMsg = fmt.Sprintf("Server error: %s", v)
		default:
			errMsg = "Unknown error code type"
		}

		klog.Infof("RPC Error - Code: %v(%T), Message: %q",
			errCode, errCode, dicts["err_msg"])

		if msg, ok := dicts["err_msg"].(string); ok {
			return nil, &SearpcError{msg}
		}
		return nil, &SearpcError{errMsg}
	}

	if ret, ok := dicts["ret"].(map[string]interface{}); ok {
		return NewSearpcObj(ret), nil
	}
	return nil, nil
}

func _fret_objlist(retStr string) ([]*SearpcObj, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return nil, &SearpcError{"Invalid response format"}
	}

	if errCode, ok := dicts["err_code"]; ok {
		var errMsg string
		switch v := errCode.(type) {
		case float64:
			errMsg = fmt.Sprintf("Server error code: %d", int(v))
		case string:
			errMsg = fmt.Sprintf("Server error: %s", v)
		default:
			errMsg = "Unknown error code type"
		}

		klog.Infof("RPC Error - Code: %v(%T), Message: %q",
			errCode, errCode, dicts["err_msg"])

		if msg, ok := dicts["err_msg"].(string); ok {
			return nil, &SearpcError{msg}
		}
		return nil, &SearpcError{errMsg}
	}

	var list []*SearpcObj
	if retList, ok := dicts["ret"].([]interface{}); ok {
		for _, item := range retList {
			if dict, ok := item.(map[string]interface{}); ok {
				list = append(list, NewSearpcObj(dict))
			}
		}
	}

	return list, nil
}

func _fret_json(retStr string) (interface{}, error) {
	var dicts map[string]interface{}
	if err := json.Unmarshal([]byte(retStr), &dicts); err != nil {
		return nil, &SearpcError{"Invalid response format"}
	}

	if errCode, ok := dicts["err_code"]; ok {
		var errMsg string
		switch v := errCode.(type) {
		case float64:
			errMsg = fmt.Sprintf("Server error code: %d", int(v))
		case string:
			errMsg = fmt.Sprintf("Server error: %s", v)
		default:
			errMsg = "Unknown error code type"
		}

		klog.Infof("RPC Error - Code: %v(%T), Message: %q",
			errCode, errCode, dicts["err_msg"])

		if msg, ok := dicts["err_msg"].(string); ok {
			return nil, &SearpcError{msg}
		}
		return nil, &SearpcError{errMsg}
	}

	return dicts["ret"], nil
}

func CreateRPCMethod(
	searpcClient interface{},
	methodName string,
	retType string,
	paramTypes []string,
) func(...interface{}) (interface{}, error) {

	return func(args ...interface{}) (interface{}, error) {
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

		if len(args) != len(paramTypes) {
			return nil, fmt.Errorf(
				"parameter count mismatch: method %s requires %d parameters, but received %d instead",
				methodName,
				len(paramTypes),
				len(args),
			)
		}

		callArgs := make([]interface{}, 0, len(args)+1)
		callArgs = append(callArgs, methodName)

		for i, arg := range args {
			if paramTypes[i] != "string" {
				callArgs = append(callArgs, arg)
				continue
			}

			if ptr, ok := arg.(*string); ok {
				if ptr == nil {
					callArgs = append(callArgs, nil)
				} else {
					callArgs = append(callArgs, *ptr)
				}
			} else if s, ok := arg.(string); ok {
				callArgs = append(callArgs, s)
			} else {
				return nil, fmt.Errorf("parameter %d expected string, got %T", i, arg)
			}
		}

		data := common.ToBytes(callArgs)

		client, ok := searpcClient.(SearpcClientInterface)
		if !ok {
			return nil, &SearpcError{"Invalid client type"}
		}

		resp, err := client.CallRemoteFuncSync(string(data))
		if err != nil {
			return nil, fmt.Errorf("RPC call failed: %v", err)
		}

		result, err := fret(resp)
		if err != nil {
			return nil, fmt.Errorf("result parsing failed: %v", err)
		}

		return result, nil
	}
}
