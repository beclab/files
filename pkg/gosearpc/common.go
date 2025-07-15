package gosearpc

// SearpcError 自定义错误类型，用于表示searpc相关的错误
// 实现error接口以便兼容Go标准错误处理机制
type SearpcError struct {
	Msg string
}

// NewSearpcError 构造函数，用于创建SearpcError实例
// 参数msg: 错误描述信息
// 返回: *SearpcError 错误实例指针
func NewSearpcError(msg string) *SearpcError {
	return &SearpcError{
		Msg: msg, // 初始化错误信息字段
	}
}

// Error 实现error接口的Error()方法
// 返回string 错误描述字符串，对应Python中的__str__方法
func (e *SearpcError) Error() string {
	return e.Msg // 返回存储的错误信息
}

/* Python代码对应关系说明：
1. 类定义对应：
   Python的class SearpcError(Exception) 对应Go的type SearpcError struct

2. 构造函数对应：
   Python的__init__方法 对应Go的NewSearpcError构造函数

3. 字符串表示对应：
   Python的__str__方法 对应Go的Error() string方法

4. 继承关系实现：
   通过实现error接口（Error()方法）使SearpcError成为有效的错误类型
   （Go没有传统继承，通过接口实现多态）

5. 错误信息存储：
   Python的self.msg 对应Go结构体的msg字段
*/
