package compress

import (
	"bufio"
	"bytes"
	"context"
	"files/pkg/files"
	"fmt"
	"github.com/nwaples/rardecode"
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

// RAR压缩器实现
type RarCompressor struct {
	binPath string
}

func (c *RarCompressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string,
	totalSize int64, callbackup func(p int, t int64), resumeIndex *int, resumBytes *int64, paused *bool) error {
	// 创建临时工作目录
	tempDir, err := os.MkdirTemp("", "rar-compress-")
	klog.Infof("Create temp dir: %s", tempDir)
	defer os.RemoveAll(tempDir)

	// 构建文件大小映射和总大小计算
	fileSizes := make(map[string]int64)
	var actualTotalSize int64
	for i, path := range fileList {
		select {
		case <-ctx.Done():
			klog.Infof("[RAR running LOG] Cancelling compressed before starting")
			return ctx.Err()
		default:
		}

		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat error: %v", err)
		}
		if info.IsDir() {
			// 空目录不计入大小但保留结构
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
			klog.Infof("[RAR running LOG] Cancelling compressed before starting")
			return ctx.Err()
		default:
		}

		targetPath := filepath.Join(tempDir, relPath)
		klog.Infof("Compress %s to %s", relPath, targetPath)

		// 关键修正1：显式处理空目录
		if strings.HasSuffix(relPath, "/") {
			if err = os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("mkdir for folder error: %v", err)
			}
			klog.Infof("Created directory: %s", targetPath)
			continue
		}

		// 常规文件处理
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

	cmd := exec.Command(
		c.binPath,
		"a", "-m5=UTF-8", "-r", "-or", "-ma5",
		"-mcp=UTF-8", "-htb",
		"-ep1",
		//"-df",
		outputPath,
		".",
	)
	cmd.Dir = tempDir

	// 进度跟踪系统
	var (
		processedBytes int64
		progressMu     sync.Mutex
	)

	// 新增管道和通道设置
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
		defer close(doneChan) // 仅在此处关闭完成通道

		for line := range outputChan {
			klog.Info("[RAR LOG] ", line)

			// 错误检测
			if strings.Contains(line, "ERROR") || strings.Contains(line, "FAILED") {
				select {
				case errorChan <- fmt.Errorf("RAR error: %s", line):
				default: // 避免阻塞
				}
				continue
			}

			// 进度解析
			if processed, ok := parseRarFileProgress(line, fileSizes); ok {
				progressMu.Lock()
				processedBytes += processed
				progress := float64(processedBytes) * 100 / float64(actualTotalSize)
				klog.Infof("[RAR parse LOG] processed: %d, processed bytes: %d, progress: %f", processed, processedBytes, progress)
				callbackup(int(progress), processed)
				progressMu.Unlock()
				select {
				case progressChan <- progress:
				default: // 避免阻塞
				}
			} else {
				klog.Warningf("[RAR parse LOG] parse progress for %s failed", line)
			}
		}
	}()

	// 修正后的管道读取协程
	progressWG.Add(1)
	go func() {
		defer progressWG.Done()
		defer close(outputChan) // 确保只关闭一次

		// 合并读取stdout和stderr
		multiReader := io.MultiReader(stdoutPipe, stderrPipe)
		scanner := bufio.NewScanner(multiReader)

		for scanner.Scan() {
			select {
			case outputChan <- cleanRarOutput(scanner.Text()):
			case <-doneChan: // 收到完成信号时退出
				return
			}
		}
	}()

	progressWG.Add(1)
	go func() {
		defer progressWG.Done()
		for {
			select {
			case progress := <-progressChan:
				klog.Infof("[RAR PROGRESS] %.2f%%", progress)
				if progress >= 100.0 {
					return
				}
			case <-doneChan: // 收到完成信号时退出
				return
			default:
			}
		}
	}()

	// 启动命令
	err = cmd.Start()
	if err != nil {
		//close(doneChan)   // 通知所有协程退出
		progressWG.Wait() // 等待所有协程完成
		return fmt.Errorf("start error: %v", err)
	}

	// 新增：监听ctx取消信号并执行清理
	go func() {
		select {
		case <-ctx.Done():
			// 清理操作：终止进程、删除输出文件
			klog.Infof("[RAR running LOG] Cancelling compressed file: %s", filepath.Base(outputPath))
			if err = cmd.Process.Kill(); err != nil {
				klog.Errorf("[RAR running LOG] Failed to kill process: %v", err)
			}
			// 删除可能已部分生成的输出文件
			if err = os.RemoveAll(outputPath); err != nil {
				klog.Errorf("[RAR running LOG] Failed to remove file: %v", err)
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

		// 头部验证
		if err := verifyRarHeader(outputPath); err != nil {
			return fmt.Errorf("header verification failed: %v", err)
		}

		klog.Info("RAR Compression Complete: 100%")
		return nil

	case <-ctx.Done(): // 新增：直接监听ctx取消
		//close(doneChan)    // 通知所有协程退出
		progressWG.Wait() // 等待所有协程完成
		return ctx.Err()  // 返回取消错误
	}
	return nil
}

//	// 进度跟踪系统
//	var (
//		processedBytes int64
//		progressMu     sync.Mutex
//	)
//
//	// 新增管道和通道设置
//	stdoutPipe, _ := cmd.StdoutPipe()
//	stderrPipe, _ := cmd.StderrPipe()
//	outputChan := make(chan string, 100)
//	errorChan := make(chan error, 1)
//	progressChan := make(chan float64, 100)
//	var progressWG sync.WaitGroup
//
//	// 日志处理协程
//	progressWG.Add(1)
//	go func() {
//		defer progressWG.Done()
//		defer close(errorChan)
//
//		for line := range outputChan {
//			klog.Info("[RAR LOG] ", line)
//
//			// 错误检测（参考rsync的错误处理）
//			if strings.Contains(line, "ERROR") || strings.Contains(line, "FAILED") {
//				errorChan <- fmt.Errorf("RAR error: %s", line)
//				continue
//			}
//
//			// 进度解析（保持原有逻辑）
//			if processed, ok := parseRarFileProgress(line, fileSizes); ok {
//				progressMu.Lock()
//				processedBytes += processed
//				progress := float64(processedBytes) * 100 / float64(actualTotalSize)
//				progressMu.Unlock()
//				progressChan <- progress
//			}
//		}
//	}()
//
//	// 修正后的管道读取协程
//	progressWG.Add(1)
//	go func() {
//		defer progressWG.Done()
//
//		// 合并读取stdout和stderr
//		multiReader := io.MultiReader(stdoutPipe, stderrPipe)
//		scanner := bufio.NewScanner(multiReader)
//
//		for scanner.Scan() {
//			line := scanner.Text()
//			// 优先处理进程结束信号
//			if cmd.Process == nil || cmd.ProcessState != nil {
//				close(outputChan)
//				return
//			}
//
//			outputChan <- line
//		}
//		close(outputChan)
//	}()
//
//	// 启动命令
//	err = cmd.Start()
//	if err != nil {
//		close(outputChan)
//		progressWG.Wait()
//		return fmt.Errorf("start error: %v", err)
//	}
//
//	// 等待命令完成
//	cmdWait := make(chan error, 1)
//	go func() {
//		cmdWait <- cmd.Wait()
//		close(cmdWait)
//	}()
//
//	// 错误处理优化
//	for {
//		select {
//		case err, ok := <-errorChan:
//			if !ok {
//				break
//			}
//			cmd.Process.Kill()
//			<-cmdWait
//			return err
//
//		case err := <-cmdWait:
//			if err != nil {
//				return fmt.Errorf("execution error: %v", err)
//			}
//			close(errorChan)
//			// 继续执行后续逻辑
//
//			progressWG.Wait()
//
//			// 头部验证
//			if err := verifyRarHeader(outputPath); err != nil {
//				return fmt.Errorf("header verification failed: %v", err)
//			}
//
//			klog.Info("RAR Compression Complete: 100%")
//			return nil
//		}
//	}
//}

//	// 执行压缩命令
//	cmd := exec.Command(
//		c.binPath,
//		"a", "-m5=UTF-8", "-r", "-or", "-ma5",
//		"-mcp=UTF-8", "-htb",
//		"-ep1",
//		outputPath,
//		".",
//	)
//	cmd.Dir = tempDir
//
//	// 进度跟踪系统
//	var (
//		processedBytes int64
//		progressMu     sync.Mutex
//	)
//
//	// 启动进度监控goroutine
//	progressChan := make(chan float64, 100)
//	var wg sync.WaitGroup
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		for progress := range progressChan {
//			klog.Infof("RAR Progress: %.2f%% (%s/%s)",
//				progress,
//				formatBytes(processedBytes),
//				formatBytes(actualTotalSize))
//		}
//	}()
//
//	// 捕获命令输出
//	//var stdout bytes.Buffer
//	//cmd.Stdout = &stdout
//	//var stderr bytes.Buffer
//	//cmd.Stderr = &stderr
//	var outputBuffer bytes.Buffer
//	cmd.Stdout = &outputBuffer
//	cmd.Stderr = &outputBuffer
//
//	// 启动命令执行
//	err = cmd.Start()
//	if err != nil {
//		close(progressChan)
//		wg.Wait()
//		return fmt.Errorf("start error: %v", err)
//	}
//
//	// 实时解析输出
//	scanner := bufio.NewScanner(&outputBuffer)
//	go func() {
//		for scanner.Scan() {
//			klog.Info("> ", scanner.Text()) // 实时输出
//
//			line := scanner.Text()
//			klog.Info("[RAR LOG] ", line)
//
//			// 关键修正2：基于文件大小的进度计算
//			if processed, ok := parseRarFileProgress(line, fileSizes); ok {
//				progressMu.Lock()
//				processedBytes += processed
//				progress := float64(processedBytes) * 100 / float64(actualTotalSize)
//				progressMu.Unlock()
//
//				progressChan <- progress
//			}
//		}
//	}()
//
//	// 等待命令完成
//	err = cmd.Wait()
//	klog.Info("FINAL OUTPUT:\n", outputBuffer.String())
//	close(progressChan)
//	wg.Wait()
//
//	if err != nil {
//		klog.Error("RAR STDERR: ", outputBuffer.String())
//		return fmt.Errorf("execution error: %v", err)
//	}
//
//	// 关键修正3：增强头部验证
//	if err := verifyRarHeader(outputPath); err != nil {
//		return fmt.Errorf("header verification failed: %v", err)
//	}
//
//	klog.Info("RAR Compression Complete: 100%")
//	return nil
//}

// 清理RAR输出中的退格符和覆盖字符
func cleanRarOutput(raw string) string {
	// 移除所有退格符和覆盖的字符
	return strings.ReplaceAll(
		strings.ReplaceAll(raw, "\b", ""),
		"\x1b[K", "",
	)
}

// 改进的进度解析函数
func parseRarFileProgress(line string, fileSizes map[string]int64) (int64, bool) {
	// 匹配添加文件行（含中文路径和百分比）
	re := regexp.MustCompile(`Adding\s+(.+?)\s+(\d+)%`)

	if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
		relPath := strings.TrimPrefix(matches[1], "./")
		percent, _ := strconv.Atoi(matches[2])

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

		// 特殊处理100%完成状态
		if percent >= 100 {
			return 0, true
		}
	}

	// 匹配最终完成状态
	if strings.Contains(line, "Done") {
		return 0, true
	}

	return 0, false
}

// 添加文件头验证
func verifyRarHeader(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, 7)
	n, _ := f.Read(header)
	if n != 7 {
		return fmt.Errorf("invalid header length")
	}

	rar4Sig := []byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x00}
	rar5Sig := []byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x01}

	if !bytes.Equal(header, rar4Sig) && !bytes.Equal(header, rar5Sig) {
		return fmt.Errorf("invalid RAR header: %x", header)
	}
	return nil
}

// 增强的进度解析
func parseRarProgress(line string) float64 {
	re := regexp.MustCompile(`\b(\d+\.?\d*)%\b`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		f, _ := strconv.ParseFloat(matches[1], 64)
		return f
	}
	return 0
}

// 查找RAR可执行文件
func findRarBin() string {
	// 环境变量优先
	if customPath := os.Getenv("RAR_BIN_PATH"); customPath != "" {
		if isExecutable(customPath) {
			return customPath
		}
	}

	// 标准路径检查
	standardPaths := []string{
		"/usr/bin/rar",
		"/usr/local/bin/rar",
		"/opt/homebrew/bin/rar",
		"C:\\Program Files\\WinRAR\\rar.exe",
		"C:\\Program Files (x86)\\WinRAR\\rar.exe",
	}

	// 合并PATH路径
	standardPaths = append(standardPaths, filepath.SplitList(os.Getenv("PATH"))...)

	// 查找可执行文件
	for _, path := range standardPaths {
		if isExecutable(path) {
			return path
		}
	}

	// 安装引导
	klog.Error("RAR not found. Installation guide:\n" +
		"Windows: https://www.win-rar.com/\n" +
		"Linux: sudo apt-get install rar\n" +
		"macOS: brew install unrar\n" +
		"Attention: no official rar compressor for linux arm64 now.")
	return ""
}

// RAR解压
func (c *RarCompressor) Uncompress(ctx context.Context, src, dest string, override bool, callbackup func(p int, t int64)) error {
	// 统一打开文件（避免多次打开）
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 第一次遍历：计算文件总数
	total, firstErr := countRarEntries(ctx, srcFile)
	if firstErr != nil {
		klog.Errorf("预扫描失败: %v", firstErr)
		return firstErr
	}

	// 重置文件指针到开头
	if _, err = srcFile.Seek(0, 0); err != nil {
		klog.Errorf("文件重置失败: %v", err)
		return err
	}

	// 第二次遍历：解压文件
	r, err := rardecode.NewReader(srcFile, "")
	if err != nil {
		klog.Errorf("创建读取器失败: %v", err)
		return err
	}
	//defer r.Close() // 确保资源释放

	return extractFiles(ctx, r, dest, total, override, callbackup)
}

// 辅助函数：计算RAR文件条目数
func countRarEntries(ctx context.Context, f *os.File) (int, error) {
	r, err := rardecode.NewReader(f, "")
	if err != nil {
		return 0, err
	}

	total := 0
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		total++

		select {
		case <-ctx.Done():
			klog.Infof("[RAR running LOG] Cancelling compressed file: %s", header.Name)
			return total, ctx.Err()
		default:
		}
	}
	return total, nil
}

// 辅助函数：执行解压操作
func extractFiles(ctx context.Context, r *rardecode.Reader, dest string, total int, override bool, callbackup func(p int, t int64)) error {
	processed := 0
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取条目失败: %v", err)
		}

		select {
		case <-ctx.Done():
			klog.Infof("[RAR running LOG] Cancelling compressed file: %s", header.Name)
			err = os.RemoveAll(dest)
			if err != nil {
				klog.Errorf("[RAR running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		// 路径安全校验（保持原有逻辑）
		fpath := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(fpath, dest+"/") {
			klog.Errorf("非法路径跳过: %s", header.Name)
			processed++
			continue
		}

		// 目录处理（保持原有逻辑）
		if header.IsDir {
			files.MkdirAllWithChown(nil, fpath, 0755)
			logProgress(processed+1, total, header.Name, callbackup)
			processed++
			continue
		}

		// 文件存在性检查（保持原有逻辑）
		if !override {
			if _, err := os.Stat(fpath); err == nil {
				klog.Infof("跳过已存在文件: %s", fpath)
				logProgress(processed+1, total, "", callbackup)
				processed++
				continue
			}
		}

		// 文件解压（关键修复：使用专用Writer）
		files.MkdirAllWithChown(nil, filepath.Dir(fpath), 0755)
		outFile, err := os.Create(fpath)
		if err != nil {
			return fmt.Errorf("创建文件失败: %v", err)
		}

		// 核心修复：使用io.CopyBuffer替代直接io.Copy
		if _, err = io.Copy(outFile, r); err != nil {
			outFile.Close()
			return fmt.Errorf("解压失败: %v", err)
		}
		outFile.Close()

		logProgress(processed+1, total, header.Name, callbackup)
		processed++
	}
	return nil
}

// 进度日志封装
func logProgress(current, total int, name string, callbackup func(p int, t int64)) {
	percent := float64(current) / float64(total) * 100
	if name != "" {
		klog.Infof("进度: %d/%d (%.2f%%) - %s", current, total, percent, name)
	} else {
		klog.Infof("进度: %d/%d (%.2f%%)", current, total, percent)
	}
	callbackup(int(percent), 0)
}
