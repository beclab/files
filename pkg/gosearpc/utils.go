// 文件名：utils.go
package gosearpc

import (
	"fmt"
	"net"
	"runtime"
	"syscall"
)

// recvall 从连接中读取指定长度的数据
// 参数：
//
//	conn - 网络连接对象
//	total - 需要读取的总字节数
//
// 返回：
//
//	[]byte - 读取到的数据
//	error - 错误信息
func recvall(conn net.Conn, total int) ([]byte, error) {
	data := make([]byte, 0, total)
	remain := total

	for remain > 0 {
		buf := make([]byte, remain)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, &NetworkError{Msg: fmt.Sprintf("Failed to read from socket: %v", err)}
		}

		if n <= 0 {
			return nil, &NetworkError{Msg: "Failed to read from socket"}
		}

		data = append(data, buf[:n]...)
		remain -= n
	}

	return data, nil
}

// sendall 向连接写入全部数据
// 参数：
//
//	conn - 网络连接对象
//	data - 需要发送的数据
//
// 返回：
//
//	error - 错误信息
func sendall(conn net.Conn, data []byte) error {
	total := len(data)
	offset := 0

	for offset < total {
		n, err := conn.Write(data[offset:])
		if err != nil {
			return &NetworkError{Msg: fmt.Sprintf("Failed to write to socket: %v", err)}
		}

		if n <= 0 {
			return &NetworkError{Msg: "Failed to write to socket"}
		}

		offset += n
	}

	return nil
}

// isWin32 判断当前操作系统是否为Windows
// 返回：
//
//	bool - 是否为Windows系统
func isWin32() bool {
	return runtime.GOOS == "windows"
}

// makeSocketCloseOnExec 设置socket的close-on-exec标志
// 参数：
//
//	conn - 网络连接对象
//
// 返回：
//
//	error - 错误信息
func makeSocketCloseOnExec(conn net.Conn) {
	// Windows 平台直接返回
	if runtime.GOOS == "windows" {
		return
	}

	// 类型断言确保是 TCP 连接
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}

	// 获取底层文件描述符（带锁）
	file, _ := tcpConn.File() // 忽略错误
	defer file.Close()        // 确保关闭文件描述符

	// 获取原始文件描述符
	fd := int(file.Fd())

	// Unix 平台直接设置 close-on-exec 标志（无返回值调用）
	syscall.CloseOnExec(fd) // 显式忽略返回值
}
