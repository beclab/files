package searpc

import (
	"encoding/json"
	"fmt"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
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
		klog.Warningf("~~~Debug log: Empty data field")
		return nil, fmt.Errorf("Empty data field")
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
		klog.Infof("~~~Debug log: Processing object %d/%d", i+1, len(objs))
		var stringMap map[string]string
		stringMap, err = obj.MapString()
		if err != nil {
			klog.Warningf("~~~Debug log: Processing object failed at index %d", i)
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
		klog.Infof("~~~Debug log: errCode: %s", errCode.(string))
		return nil, &SearpcError{dicts["err_msg"].(string)}
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
		klog.Infof("~~~Debug log: errCode: %s", errCode.(string))
		return nil, &SearpcError{dicts["err_msg"].(string)}
	}

	var list []*SearpcObj
	if retList, ok := dicts["ret"].([]interface{}); ok {
		for _, item := range retList {
			if dict, ok := item.(map[string]interface{}); ok {
				list = append(list, NewSearpcObj(dict))
			}
		}
	}

	for key, value := range list {
		klog.Infof("~~~Debug log: Key: %v, Value: %v (Type: %T)\n", key, value, value)
	}
	return list, nil
}

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

func CreateRPCMethod(
	searpcClient interface{},
	methodName string,
	retType string,
	paramTypes []string,
) func(...interface{}) (interface{}, error) {
	klog.Infof("~~~Debug log: Creating RPC method %s with return type %s and %d parameters", methodName, retType, len(paramTypes))

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
			klog.Infof("~~~Debug log: Parameter mismatch for %s - expected %d, got %d",
				methodName, len(paramTypes), len(args))
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

		data, err := json.Marshal(callArgs)
		if err != nil {
			klog.Infof("~~~Debug log: Failed to marshal arguments for %s: %v", methodName, err)
			return nil, fmt.Errorf("failed to marshal arguments: %v", err)
		}

		client, ok := searpcClient.(SearpcClientInterface)
		if !ok {
			klog.Infof("~~~Debug log: client does not implement SearpcClientInterface")
			return nil, &SearpcError{"Invalid client type"}
		}

		klog.Infof("~~~Debug log: Calling remote function %s via named pipe", methodName)
		resp, err := client.CallRemoteFuncSync(string(data))
		if err != nil {
			klog.Infof("~~~Debug log: RPC call failed for %s: %v", methodName, err)
			return nil, fmt.Errorf("RPC call failed: %v", err)
		}

		klog.Infof("~~~Debug log: Raw response for %s: %s", methodName, resp)
		result, err := fret(resp)
		if err != nil {
			klog.Infof("~~~Debug log: Result parsing failed for %s: %v", methodName, err)
			return nil, fmt.Errorf("result parsing failed: %v", err)
		}

		klog.Infof("~~~Debug log: Successfully processed %s result: %+v (type: %T)", methodName, result, result)
		return result, nil
	}
}
