package gosearpc

// NetworkError 自定义网络错误类型，对应Python中的NetworkError类
type NetworkError struct {
	Msg string // 错误信息描述，对应Python中的self.msg
}

// NewNetworkError 构造函数，用于创建NetworkError实例
// 参数msg: 错误描述信息
// 返回: *NetworkError 错误实例指针
func NewNetworkError(msg string) *NetworkError {
	return &NetworkError{
		Msg: msg, // 初始化错误信息字段
	}
}

// Error 实现error接口的Error()方法，对应Python中的__str__方法
// 返回string 错误描述字符串
func (e *NetworkError) Error() string {
	return e.Msg // 返回存储的错误信息
}

/* Python代码对应关系说明：
1. 类定义对应：
   Python的class NetworkError(Exception) 对应Go的type NetworkError struct

2. 构造函数对应：
   Python的__init__方法 对应Go的NewNetworkError构造函数

3. 字符串表示对应：
   Python的__str__方法 对应Go的Error() string方法

4. 继承关系实现：
   通过实现error接口（Error()方法）使NetworkError成为有效的错误类型
   （Go没有传统继承，通过接口实现多态）

5. 错误信息存储：
   Python的self.msg 对应Go结构体的Msg字段
*/
