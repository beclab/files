package gosearpc

//import (
//	"encoding/json"
//	"fmt"
//)
//
//type SearpcService struct {
//	Name      string
//	FuncTable map[string]interface{}
//}
//
//type SearpcServer struct {
//	Services map[string]*SearpcService
//}
//
//type errorResponse struct {
//	ErrCode int    `json:"err_code"`
//	ErrMsg  string `json:"err_msg"`
//}
//
//type successResponse struct {
//	Ret interface{} `json:"ret"`
//}
//
//func NewSearpcServer() *SearpcServer {
//	return &SearpcServer{
//		Services: make(map[string]*SearpcService),
//	}
//}
//
//func (s *SearpcServer) CreateService(svcname string) {
//	s.Services[svcname] = &SearpcService{
//		Name:      svcname,
//		FuncTable: make(map[string]interface{}),
//	}
//}
//
//func (s *SearpcServer) RegisterFunction(svcname string, fn interface{}, fname string) {
//	service, exists := s.Services[svcname]
//	if !exists {
//		panic(fmt.Sprintf("Service %s not exists", svcname))
//	}
//
//	if fname == "" {
//		fname = "anonymous_function"
//	}
//	service.FuncTable[fname] = fn
//}
//
//func (s *SearpcServer) callFunction(svcname string, fcallstr string) (interface{}, error) {
//	var argv []interface{}
//	if err := json.Unmarshal([]byte(fcallstr), &argv); err != nil {
//		return nil, &SearpcError{Msg: "bad call str: " + err.Error()}
//	}
//
//	service, exists := s.Services[svcname]
//	if !exists {
//		return nil, &SearpcError{Msg: "Service not exists"}
//	}
//
//	if len(argv) == 0 {
//		return nil, &SearpcError{Msg: "Empty function name"}
//	}
//	fname, ok := argv[0].(string)
//	if !ok {
//		return nil, &SearpcError{Msg: "Invalid function name type"}
//	}
//
//	fn, exists := service.FuncTable[fname]
//	if !exists {
//		return nil, &SearpcError{Msg: "No such function " + fname}
//	}
//
//	return fn, nil
//}
//
//func (s *SearpcServer) CallFunction(svcname string, fcallstr string) string {
//	defer func() {
//		if r := recover(); r != nil {
//			errMsg := fmt.Sprintf("Internal error: %v", r)
//			resp, _ := json.Marshal(errorResponse{
//				ErrCode: 555,
//				ErrMsg:  errMsg,
//			})
//			fmt.Println(string(resp))
//		}
//	}()
//
//	retVal, err := s.callFunction(svcname, fcallstr)
//	if err != nil {
//		resp, _ := json.Marshal(errorResponse{
//			ErrCode: 555,
//			ErrMsg:  err.Error(),
//		})
//		return string(resp)
//	}
//
//	resp, _ := json.Marshal(successResponse{
//		Ret: retVal,
//	})
//	return string(resp)
//}
//
//var GlobalSearpcServer = NewSearpcServer()
