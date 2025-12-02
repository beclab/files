package compress

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ZIP压缩器（保持原有实现）
type ZipCompressor struct{}

//func getFileSize(path string) int64 {
//	info, err := os.Stat(path)
//	if err != nil {
//		return 0
//	}
//	return info.Size()
//}

func (c *ZipCompressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string,
	totalSize int64, updateProgress func(p int, t int64),
	getPauseInfo func() (int, int64),
	setPauseInfo func(i int, b int64),
	getPaused func() bool,
) error {
	resumeIndex, resumeBytes := getPauseInfo()
	klog.Infof("[ZIP running LOG] got pause info: resumeIndex: %d, resumeBytes: %d", resumeIndex, resumeBytes)

	// 初始化或恢复进度跟踪变量
	processedBytes := int64(0)
	if resumeBytes != int64(0) {
		processedBytes = resumeBytes
	}
	lastReported := -1.0
	reportInterval := 0.5

	currentFileIndex := 0
	if resumeIndex != 0 {
		currentFileIndex = resumeIndex
	}
	klog.Infof("[ZIP running LOG] processedBytes: %d, currentFileIndex: %d", processedBytes, currentFileIndex)

	select {
	case <-ctx.Done():
		if getPaused() {
			klog.Infof("[ZIP running LOG] Paused compressing before starting")
			setPauseInfo(currentFileIndex, processedBytes)
		} else {
			klog.Infof("[ZIP running LOG] Cancelled compressing before starting")
		}
		return ctx.Err()
	default:
	}

	zipFile, err := os.Create(outputPath)
	if err != nil {
		klog.Errorf("Failed to create zip file: %v", err)
		return err
	}
	defer func() {
		if err = zipFile.Close(); err != nil {
			klog.Errorf("Failed to close zip file: %v", err)
		}
	}()

	zipWriter := zip.NewWriter(zipFile)
	defer func() {
		if err = zipWriter.Close(); err != nil {
			klog.Errorf("Failed to close zip writer: %v", err)
		}
	}()

	for index := currentFileIndex; index < len(fileList); index++ {
		filePath := fileList[index]

		klog.Infof("[ZIP running LOG] index: %d, filePath: %s", index, filePath)
		klog.Infof("[ZIP running LOG] filePath: %s", filePath)

		select {
		case <-ctx.Done():
			if getPaused() {
				// 保留已压缩内容，仅中断后续处理
				klog.Infof("Compression interrupted at file %d", index)
				setPauseInfo(index, processedBytes)
				return ctx.Err()
			} else {
				klog.Infof("[ZIP running LOG] Cancelled compressing file: %s", filepath.Base(filePath))
				err = os.RemoveAll(outputPath)
				if err != nil {
					klog.Errorf("[ZIP running LOG] Failed to remove file: %v", err)
				}
				return ctx.Err()
			}
		default:
		}

		relPath := relPathList[index]
		//fileSize := getFileSize(filePath) // 保留原有文件大小获取逻辑

		//klog.Infof("Processing file: %s (offset: %d, size: %d)", filePath, processedBytes, fileSize)

		err = addFileToZip(
			zipWriter,
			fileList[index],
			relPath,
			totalSize,
			&processedBytes,
			&lastReported,
			reportInterval,
			updateProgress, // progressWrapper, // 使用封装后的回调
		)
		klog.Infof("[ZIP running LOG] after adding %s", filePath)

		if err != nil {
			if getPaused() {
				klog.Infof("Compression paused at file %d", index)
				return ctx.Err()
			} else {
				klog.Errorf("Compression failed: %v", err)
				return err
			}
		}

		// 保存当前处理位置到外部参数（关键状态同步点）
		setPauseInfo(index, processedBytes)
		klog.Infof("[ZIP running LOG] index: %d, processedBytes: %d", index, processedBytes)

		klog.Infof("[ZIP running LOG] for pause test, will sleep 5 seconds...")
		time.Sleep(5 * time.Second)
	}

	// 最终进度报告（保留原有逻辑）
	if totalSize > 0 {
		progress := 100.0
		klog.Infof("Compression completed: %.2f%%", progress)
		updateProgress(int(progress), 0)
	}
	return nil
}

//func (c *ZipCompressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string,
//	totalSize int64, callbackup func(p int, t int64), resumeIndex *int, resumeBytes *int64, paused *bool) error {
//	// 保持原有addFileToZip实现
//	// 进度通过全局变量processedBytes和lastReported同步更新到klog
//	// 初始化进度跟踪变量
//	processedBytes := int64(0)
//	lastReported := -1.0  // 初始化为-1确保首次触发
//	reportInterval := 0.5 // 进度报告阈值(百分比)
//
//	select {
//	case <-ctx.Done():
//		klog.Infof("[ZIP running LOG] Cancelled compressing before starting")
//		return ctx.Err()
//	default:
//	}
//
//	zipFile, err := os.Create(outputPath)
//	if err != nil {
//		klog.Errorf("Failed to create zip file: %v", err)
//		return err
//	}
//	defer zipFile.Close()
//
//	zipWriter := zip.NewWriter(zipFile)
//	defer zipWriter.Close()
//
//	for index, filePath := range fileList {
//		select {
//		case <-ctx.Done():
//			klog.Infof("[ZIP running LOG] Cancelled compressing file: %s", filepath.Base(filePath))
//			err = os.RemoveAll(outputPath)
//			if err != nil {
//				klog.Errorf("[ZIP running LOG] Failed to remove file: %v", err)
//			}
//			return ctx.Err()
//		default:
//		}
//
//		relPath := relPathList[index]
//		klog.Infof("Processing file: %s", filePath)
//		err = addFileToZip(
//			zipWriter,
//			filePath,
//			relPath,
//			totalSize,
//			&processedBytes,
//			&lastReported,
//			reportInterval,
//			callbackup,
//		)
//		if err != nil {
//			klog.Errorf("Compression failed: %v", err)
//			return err
//		}
//	}
//
//	// 最终进度报告
//	if totalSize > 0 {
//		//progress := float64(processedBytes) / float64(totalSize) * 100
//		progress := 100.0
//		klog.Infof("Compression completed: %.2f%%", progress)
//		callbackup(int(progress), 0)
//	}
//	return nil
//}

func addFileToZip(zw *zip.Writer, srcPath, relPath string, totalSize int64,
	processedBytes *int64, lastReported *float64, reportInterval float64, updateProgress func(p int, t int64)) error {

	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("stat error: %v", err)
	}

	// 关键修改1：处理目录类型
	if info.IsDir() {
		if !strings.HasSuffix(relPath, "/") {
			relPath += "/"
		}
		// 创建目录占位符（必须以/结尾）
		_, err = zw.Create(relPath)
		if err != nil {
			klog.Errorf("Failed to create directory entry: %v", err)
			return err
		}
		// 关键修改2：更新进度（空目录也计入处理）
		*processedBytes += 4096 // 占位但计入进度
		progress := float64(*processedBytes) * 100 / float64(totalSize)
		//if shouldReport(progress, *lastReported, reportInterval) {
		klog.Infof("Progress: %.2f%% (Directory: %s)", progress, relPath)
		*lastReported = progress
		updateProgress(int(progress), 4096)
		//}
		return nil // 目录处理完成直接返回
	}

	// 创建ZIP文件头（关键修正：使用NewEntry避免路径问题）
	klog.Infof("Adding file: %s -> %s", srcPath, relPath)
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = relPath
	header.Method = zip.Deflate

	// 关键修正1：使用Create()代替CreateHeader()确保路径正确处理
	fileInZip, err := zw.Create(header.Name)
	if err != nil {
		// 添加详细错误日志
		klog.Errorf("Zip create failed: %s (relPath: %s)", err, relPath)
		return fmt.Errorf("failed to create zip entry: %v", err)
	}

	// 关键修正2：延迟打开源文件直到确认ZIP条目创建成功
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 动态缓冲区
	buf := make([]byte, bufferSize(info.Size()))
	bytesRead := 0
	progress := 0.0

	for {
		n, err := srcFile.Read(buf)
		if err != nil && err != io.EOF {
			klog.Errorf("Read error: %v", err)
			return err
		}

		if n > 0 {
			// 关键修正3：捕获写入错误的详细上下文
			_, err = fileInZip.Write(buf[:n])
			if err != nil {
				// 添加写入错误的详细诊断信息
				klog.Errorf("Write error: %v (offset: %d, size: %d, path: %s)",
					err, bytesRead, n, relPath)
				return fmt.Errorf("write failed at offset %d: %v", bytesRead, err)
			}

			// 更新全局进度
			*processedBytes += int64(n)
			bytesRead += n
			progress = float64(*processedBytes) * 100 / float64(totalSize)

			// 阈值触发式进度报告
			if shouldReport(progress, *lastReported, reportInterval) {
				klog.Infof("Progress: %.2f%% (%s/%s)",
					progress,
					formatBytes(*processedBytes),
					formatBytes(totalSize))
				*lastReported = progress
				updateProgress(int(progress), int64(bytesRead))
				bytesRead = 0
			}
		}

		if err == io.EOF {
			if bytesRead != 0 {
				klog.Infof("Progress: %.2f%% (%s/%s)",
					progress,
					formatBytes(*processedBytes),
					formatBytes(totalSize))
				*lastReported = progress
				updateProgress(int(progress), int64(bytesRead))
				bytesRead = 0
			}
			break
		}
	}

	// 关键修正4：显式刷新ZIP条目
	if closer, ok := fileInZip.(io.Closer); ok {
		closer.Close()
	}

	return nil
}

// ZIP解压
func (c *ZipCompressor) Uncompress(
	ctx context.Context,
	src, dest string,
	override bool,
	callbackup func(p int, t int64),
) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	total := len(r.File)
	processed := 0

	for _, f := range r.File {
		select {
		case <-ctx.Done():
			klog.Infof("[ZIP running LOG] Cancelling uncompressed file: %s", f.Name)
			err = os.RemoveAll(dest)
			if err != nil {
				klog.Errorf("[ZIP running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, dest+"/") {
			return fmt.Errorf("非法路径: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			processed++
			klog.Infof("进度: %d/%d (%.2f%%) - %s",
				processed, total,
				float64(processed)/float64(total)*100,
				f.Name)
			callbackup(int(float64(processed)/float64(total)*100), 0)
			continue
		}

		if !override {
			if _, err := os.Stat(fpath); err == nil {
				klog.Infof("跳过已存在的文件: %s", fpath)
				processed++
				klog.Infof("进度: %d/%d (%.2f%%)",
					processed, total,
					float64(processed)/float64(total)*100)
				callbackup(int(float64(processed)/float64(total)*100), 0)
				continue
			}
		}

		os.MkdirAll(filepath.Dir(fpath), 0755)

		out, err := os.Create(fpath)
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()

		if err != nil {
			return err
		}

		processed++
		klog.Infof("进度: %d/%d (%.2f%%) - %s",
			processed, total,
			float64(processed)/float64(total)*100,
			f.Name)
		callbackup(int(float64(processed)/float64(total)*100), 0)
	}
	return nil
}
