package compress

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// 7-Zip压缩器（使用系统二进制）
type SevenZipCompressor struct {
	binPath string
	//currentFile string
	//fileMu      sync.Mutex
}

func (c *SevenZipCompressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string, totalSize int64, callbackup func(p int, t int64)) error {
	// 创建临时工作目录
	tempDir, err := os.MkdirTemp("", "7z-compress-")
	klog.Infof("Create temp dir: %s", tempDir)
	defer os.RemoveAll(tempDir)

	// 构建文件大小映射和总大小计算
	fileSizes := make(map[string]int64)
	var actualTotalSize int64
	for i, path := range fileList {
		select {
		case <-ctx.Done():
			klog.Infof("[7Z running LOG] Cancelling compressed before starting")
			return ctx.Err()
		default:
		}

		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat error: %v", err)
		}
		if info.IsDir() {
			fileSizes[relPathList[i]] = 0
			continue
		}
		actualTotalSize += info.Size()
		fileSizes[relPathList[i]] = info.Size()
	}

	// 复制文件到临时目录（保持相对路径结构）
	for i, relPath := range relPathList {
		select {
		case <-ctx.Done():
			klog.Infof("[7Z running LOG] Cancelling compressed before starting")
			return ctx.Err()
		default:
		}

		targetPath := filepath.Join(tempDir, relPath)
		klog.Infof("Compress %s to %s", relPath, targetPath)

		if strings.HasSuffix(relPath, "/") {
			if err = os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("mkdir for folder error: %v", err)
			}
			klog.Infof("Created directory: %s", targetPath)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("mkdir for file error: %v", err)
		}
		src, _ := os.Open(fileList[i])
		dst, _ := os.Create(targetPath)
		io.Copy(dst, src)
		src.Close()
		dst.Close()
		klog.Infof("Copied file: %s -> %s", fileList[i], targetPath)
	}

	// 构建7z命令
	args := []string{
		"a",
		"-y",
		"-bb3",
		"-spf2", // 保留完整路径结构
		outputPath,
		".",
	}
	klog.Infof("7z cmd: %s, %v", c.binPath, args)

	cmd := exec.Command(c.binPath, args...)
	cmd.Dir = tempDir

	// 通道和同步工具
	var (
		processedBytes int64
		progressMu     sync.Mutex
	)

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	outputChan := make(chan string, 100)
	errorChan := make(chan error, 1)
	doneChan := make(chan struct{}) // 新增完成通道
	progressChan := make(chan float64, 100)
	var progressWG sync.WaitGroup

	// 日志处理协程
	progressWG.Add(1)
	go func() {
		defer progressWG.Done()
		defer close(doneChan)

		for line := range outputChan {
			klog.Infof("[7Z LOG] %s", line)

			// 错误检测
			if strings.Contains(line, "ERROR") || strings.Contains(line, "FAILED") {
				select {
				case errorChan <- fmt.Errorf("7z error: %s", line):
				default:
				}
				continue
			}

			// 精准匹配文件完成行
			if isFileComplete(line) {
				relPath := extractFilePath(line)
				klog.Infof("[7Z parse LOG] relPath: %s", relPath)
				if size, exists := fileSizes[relPath]; exists {
					progressMu.Lock()
					processedBytes += size
					progress := float64(processedBytes) * 100 / float64(totalSize)
					if progress > 100.0 {
						progress = 100
					}
					callbackup(int(progress), size)
					klog.Infof("[7Z parse LOG] processed bytes: %d, totalSize: %d, progress: %f", processedBytes, totalSize, progress)
					progressMu.Unlock()
					select {
					case progressChan <- progress:
					default: // 避免阻塞
					}
				} else {
					klog.Infof("[7Z parse LOG] no relpath for line %s", line)
				}
			} else if line == "Everything is Ok" {
				progress := 100.0
				klog.Infof("[7Z parse LOG] Everything is Ok. Progress: %f", progress)
				select {
				case progressChan <- progress:
				default: // 避免阻塞
				}
			} else {
				klog.Infof("[7Z parse LOG] What's this?! %s", line)
			}
		}
	}()

	// 管道读取协程
	progressWG.Add(1)
	go func() {
		defer progressWG.Done()
		defer close(outputChan) // 确保只关闭一次

		// 合并读取stdout和stderr
		multiReader := io.MultiReader(stdoutPipe, stderrPipe)
		scanner := bufio.NewScanner(multiReader)

		for scanner.Scan() {
			select {
			case outputChan <- clean7zOutput(scanner.Text()):
			case <-doneChan: // 收到完成信号时退出
				return
			}
		}
	}()

	// 进度消费协程
	progressWG.Add(1)
	go func() {
		defer progressWG.Done()
		for {
			select {
			case progress := <-progressChan:
				klog.Infof("[7Z PROGRESS] %.2f%%", progress)
				if progress >= 100.0 {
					return
				}
			case <-doneChan:
				return
			}
		}
	}()

	// 启动命令
	err = cmd.Start()
	if err != nil {
		progressWG.Wait()
		return fmt.Errorf("start error: %v", err)
	}

	// 新增：监听ctx取消信号并执行清理
	go func() {
		select {
		case <-ctx.Done():
			// 清理操作：终止进程、删除输出文件
			klog.Infof("[7Z running LOG] Cancelling compressed file: %s", filepath.Base(outputPath))
			if err = cmd.Process.Kill(); err != nil {
				klog.Errorf("[7Z running LOG] Failed to kill process: %v", err)
			}
			// 删除可能已部分生成的输出文件
			if err = os.RemoveAll(outputPath); err != nil {
				klog.Errorf("[7Z running LOG] Failed to remove file: %v", err)
			}
			//close(doneChan)   // 通知所有协程退出
			progressWG.Wait() // 等待所有协程完成
		case <-doneChan: // 正常完成或错误时退出
			return
		}
	}()

	waitChan := make(chan error, 1)
	go func() {
		waitChan <- cmd.Wait()
	}()

	// 错误处理和等待
	select {
	case err, ok := <-errorChan:
		if !ok {
			break
		}
		cmd.Process.Kill() // 终止进程
		cmd.Wait()         // 等待进程结束
		//close(doneChan)    // 通知所有协程退出
		progressWG.Wait() // 等待所有协程完成
		return err

	case waitErr := <-waitChan: // 替换原来的 case <- cmd.Wait()
		if waitErr != nil {
			return fmt.Errorf("execution error: %v", waitErr)
		}
		//close(doneChan)   // 正常完成时关闭通道
		progressWG.Wait() // 等待所有协程完成

		klog.Info("7Z Compression Complete: 100%")
		return nil

	case <-ctx.Done(): // 新增：直接监听ctx取消
		//close(doneChan)    // 通知所有协程退出
		progressWG.Wait() // 等待所有协程完成
		return ctx.Err()  // 返回取消错误
	}
	return nil
}

// 清理7z输出中的控制字符
func clean7zOutput(raw string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(raw, "\b", ""),
		"\x1b[K", "",
	)
}

// 精准文件完成行检测（仅匹配+开头）
func isFileComplete(line string) bool {
	// 修正后的正则：仅匹配以+开头的行
	fileCompleteRe := regexp.MustCompile(`^\+ [^\r\n]+$`)
	return fileCompleteRe.MatchString(line)
}

// 智能路径提取与规范化
func extractFilePath(line string) string {
	// 仅处理+开头的行
	if !strings.HasPrefix(line, "+ ") {
		return ""
	}

	// 去除+前缀并规范化
	cleanLine := line[2:]
	cleanLine = strings.ReplaceAll(cleanLine, "\\ ", " ") // 仅还原转义空格
	return filepath.Clean(cleanLine)
}

// 解析7z进度输出
func parse7zFileProgress(line string, fileSizes map[string]int64) (int64, bool) {
	// 匹配文件添加操作（A filename 或 + path/filename）
	re := regexp.MustCompile(`^(A|\\+)\\s+(.+)$`)
	if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
		relPath := matches[2]

		// 尝试三种路径匹配方式
		if size, exists := fileSizes[relPath]; exists {
			return size, true
		}
		if size, exists := fileSizes[filepath.Base(relPath)]; exists {
			return size, true
		}
		if size, exists := fileSizes[strings.ReplaceAll(relPath, " ", "\\ ")]; exists {
			return size, true
		}
	}

	// 匹配总字节数信息（如106842567 bytes）
	totalRe := regexp.MustCompile(`(\\d+) bytes`)
	if matches := totalRe.FindStringSubmatch(line); len(matches) >= 2 {
		totalSize, _ := strconv.ParseInt(matches[1], 10, 64)
		return totalSize, true
	}

	return 0, false
}

func find7zBin() string {
	// 优先级1：环境变量覆盖
	if customPath := os.Getenv("SEVENZIP_BIN_PATH"); customPath != "" {
		if isExecutable(customPath) {
			klog.Infof("Using custom 7z path from environment: %s", customPath)
			return customPath
		}
		klog.Warningf("Custom 7z path %s not executable", customPath)
	}

	// 优先级2：标准安装路径（Linux/macOS）
	standardPaths := []string{
		"/usr/bin/7z",
		"/usr/local/bin/7z",
		"/opt/homebrew/bin/7z", // macOS ARM
		"/usr/bin/7za",         // 轻量版
		"/usr/local/bin/7za",
		"/snap/bin/7z", // Snap安装路径
	}

	// 优先级3：PATH环境变量
	standardPaths = append(standardPaths, filepath.SplitList(os.Getenv("PATH"))...)

	// 去重处理
	uniquePaths := deduplicatePaths(standardPaths)

	// 查找可执行文件
	for _, path := range uniquePaths {
		if isExecutable(path) {
			klog.Infof("Found 7z at: %s", path)
			return path
		}
	}

	// 安装建议
	klog.Error("7z not found. Installation guide:\n" +
		"Debian/Ubuntu: sudo apt-get install p7zip-full\n" +
		"RHEL/CentOS: sudo yum install p7zip\n" +
		"macOS: brew install p7zip")

	return ""
}

func (c *SevenZipCompressor) Uncompress(
	ctx context.Context,
	src, dest string,
	override bool,
	callbackup func(p int, t int64),
) error {
	// 创建双向取消信号通道
	cancelChan := make(chan struct{}, 1)
	defer close(cancelChan)

	// 执行7z解压命令（启用详细进度输出）
	cmd := exec.Command("7z", "x", "-y", "-o"+dest, src,
		"-bb3",  // 启用详细进度输出
		"-bsp1", // 标准输出进度
	)
	cmd.Dir = filepath.Dir(src)

	// 设置进度捕获管道
	pipe, _ := cmd.StdoutPipe()
	scanner := bufio.NewScanner(pipe)

	// 启动命令执行
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("启动解压失败: %v", err)
	}
	// 确保进程被终止
	defer func() {
		if err := cmd.Process.Kill(); err != nil {
			klog.Errorf("终止进程失败: %v", err)
		}
	}()

	// 同步工具确保资源释放
	var wg sync.WaitGroup
	wg.Add(1)

	// 进度解析协程
	go func() {
		defer func() {
			if err := scanner.Err(); err != nil {
				klog.Errorf("扫描输出错误: %v", err)
			}
		}()

		// 正则表达式：匹配所有百分比数值
		percentRe := regexp.MustCompile(`(\d+)%`)

		for scanner.Scan() {
			select {
			case <-ctx.Done(): // 监听外部上下文取消
				klog.Info("检测到外部取消请求")
				// 发送内部取消信号
				select {
				case cancelChan <- struct{}{}:
				default:
				}
				return
			default:
			}

			line := scanner.Text()
			klog.Infof("[7Z LOG] %s", line)

			// 查找所有百分比匹配
			matches := percentRe.FindAllStringSubmatch(line, -1)
			if len(matches) > 0 {
				// 取最后一个匹配的百分比
				lastMatch := matches[len(matches)-1]
				percent, _ := strconv.Atoi(lastMatch[1])
				currentProgress := float64(percent)

				// 每次解析到百分比时立即输出进度
				klog.Infof("[7Z PROGRESS] %.2f%%", currentProgress)
				callbackup(int(currentProgress), 0)
			}
		}
	}()

	// 监听上下文取消和命令完成
	select {
	case <-cancelChan: // 内部取消信号
		klog.Info("处理内部取消请求")
		// 执行清理操作
		if err = os.RemoveAll(dest); err != nil {
			klog.Errorf("[7Z running LOG] Failed to remove file: %v", err)
		}
		return ctx.Err()

	case <-ctx.Done():
		klog.Info("[7Z running LOG] Cancelled uncompressing")
		// 终止进程
		if err = cmd.Process.Kill(); err != nil {
			klog.Errorf("[7Z running LOG] Failed to kill process: %v", err)
		}
		err = os.RemoveAll(dest)
		if err != nil {
			klog.Errorf("[7Z running LOG] Failed to remove file: %v", err)
		}
		//// 等待进度解析协程退出
		//wg.Wait()
		return ctx.Err()

	case err = <-waitForCmd(cmd): // 自定义等待函数
		if err != nil {
			return fmt.Errorf("解压失败: %v", err)
		}
	}

	// 等待命令完成
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("解压失败: %v", err)
	}

	return nil
}

// 自定义等待命令完成函数
func waitForCmd(cmd *exec.Cmd) chan error {
	errChan := make(chan error, 1)
	go func() {
		errChan <- cmd.Wait()
	}()
	return errChan
}
