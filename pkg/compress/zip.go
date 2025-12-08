package compress

import (
	"bytes"
	"context"
	"fmt"
	"github.com/klauspost/compress/zip"
	"io"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ZIP压缩器（保持原有实现）
type ZipCompressor struct{}

//func (c *ZipCompressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string,
//	totalSize int64, t *TaskFuncs) error {
//	klog.Infof("[ZIP running LOG] task: %+v", t)
//	resumeIndex, resumeBytes := t.GetCompressPauseInfo()
//	klog.Infof("[ZIP running LOG] got pause info: resumeIndex: %d, resumeBytes: %d", resumeIndex, resumeBytes)
//
//	processedBytes := int64(0)
//	if resumeBytes != 0 {
//		processedBytes = resumeBytes
//	}
//	lastReported := -1.0
//	reportInterval := 0.5
//
//	currentFileIndex := 0
//	if resumeIndex != 0 {
//		currentFileIndex = resumeIndex
//	}
//	klog.Infof("[ZIP running LOG] processedBytes: %d, currentFileIndex: %d", processedBytes, currentFileIndex)
//
//	select {
//	case <-ctx.Done():
//		if t.GetCompressPaused() {
//			klog.Infof("[ZIP running LOG] Paused compressing before starting")
//			t.SetCompressPauseInfo(currentFileIndex, processedBytes)
//		} else {
//			klog.Infof("[ZIP running LOG] Cancelled compressing before starting")
//		}
//		return ctx.Err()
//	default:
//	}
//
//	// 关键修改1：使用追加模式打开文件并定位到文件末尾
//	f, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE, 0666)
//	if err != nil {
//		klog.Errorf("Failed to open zip file: %v", err)
//		return err
//	}
//	defer f.Close()
//
//	// 关键修改2：正确处理现有ZIP文件结构
//	stat, _ := f.Stat()
//	var zw *zip.Writer
//	if stat.Size() > 0 {
//		// 读取现有ZIP结构但不复制内容
//		_, err := zip.NewReader(f, stat.Size())
//		if err != nil {
//			klog.Infof("Creating new zip file")
//			f.Seek(0, io.SeekStart)
//			zw = zip.NewWriter(f)
//		} else {
//			klog.Infof("Appending to existing zip file")
//			f.Seek(0, io.SeekEnd) // 定位到文件末尾
//			zw = zip.NewWriter(f)
//		}
//	} else {
//		klog.Infof("Creating new zip file")
//		zw = zip.NewWriter(f)
//	}
//	defer zw.Close()
//
//	// 关键修改3：跳过已处理的文件
//	processedFiles := make(map[string]bool)
//	if resumeIndex > 0 {
//		for i := 0; i < resumeIndex; i++ {
//			processedFiles[relPathList[i]] = true
//		}
//	}
//
//	for index := currentFileIndex; index < len(fileList); index++ {
//		if processedFiles[relPathList[index]] {
//			klog.Infof("Skipping already processed file: %s", relPathList[index])
//			continue
//		}
//
//		filePath := fileList[index]
//		klog.Infof("[ZIP running LOG] index: %d, filePath: %s", index, filePath)
//
//		select {
//		case <-ctx.Done():
//			if t.GetCompressPaused() {
//				klog.Infof("Compression interrupted at file %d", index)
//				t.SetCompressPauseInfo(index, processedBytes)
//				return ctx.Err()
//			} else {
//				klog.Infof("[ZIP running LOG] Cancelled compressing file: %s", filepath.Base(filePath))
//				return ctx.Err()
//			}
//		default:
//		}
//
//		relPath := relPathList[index]
//		err = addFileToZip(
//			zw,
//			fileList[index],
//			relPath,
//			totalSize,
//			&processedBytes,
//			&lastReported,
//			reportInterval,
//			t,
//		)
//		klog.Infof("[ZIP running LOG] after adding %s", filePath)
//
//		if err != nil {
//			if t.GetCompressPaused() {
//				klog.Infof("Compression paused at file %d", index)
//				return ctx.Err()
//			} else {
//				klog.Errorf("Compression failed: %v", err)
//				return err
//			}
//		}
//
//		t.SetCompressPauseInfo(index, processedBytes)
//		klog.Infof("[ZIP running LOG] index: %d, processedBytes: %d", index, processedBytes)
//		time.Sleep(5 * time.Second)
//	}
//
//	if totalSize > 0 {
//		progress := 100.0
//		klog.Infof("Compression completed: %.2f%%", progress)
//		t.UpdateProgress(int(progress), 0)
//	}
//	return nil
//}

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

//func addFileToZip(zw *zip.Writer, srcPath, relPath string, totalSize int64,
//	processedBytes *int64, lastReported *float64, reportInterval float64, t *TaskFuncs) error {
//
//	info, err := os.Stat(srcPath)
//	if err != nil {
//		klog.Errorf("stat error: %v", err)
//		return fmt.Errorf("stat error: %v", err)
//	}
//
//	if info.IsDir() {
//		if !strings.HasSuffix(relPath, "/") {
//			relPath += "/"
//		}
//		_, err = zw.Create(relPath)
//		if err != nil {
//			klog.Errorf("Failed to create directory entry: %v", err)
//			return err
//		}
//		*processedBytes += 4096
//		progress := float64(*processedBytes) * 100 / float64(totalSize)
//		klog.Infof("Progress: %.2f%% (Directory: %s)", progress, relPath)
//		*lastReported = progress
//		t.UpdateProgress(int(progress), 4096)
//		return nil
//	}
//
//	klog.Infof("Adding file: %s -> %s", srcPath, relPath)
//	header, err := zip.FileInfoHeader(info)
//	if err != nil {
//		klog.Errorf("Failed to create zip header: %v", err)
//		return err
//	}
//	header.Name = relPath
//	header.Method = zip.Deflate
//
//	fileInZip, err := zw.Create(header.Name)
//	if err != nil {
//		klog.Errorf("Zip create failed: %s (relPath: %s)", err, relPath)
//		return fmt.Errorf("failed to create zip entry: %v", err)
//	}
//
//	srcFile, err := os.Open(srcPath)
//	if err != nil {
//		klog.Errorf("Failed to open zip file: %v", err)
//		return err
//	}
//	defer srcFile.Close()
//
//	buf := make([]byte, 4096)
//	bytesRead := 0
//	progress := 0.0
//
//	for {
//		n, err := srcFile.Read(buf)
//		if err != nil && err != io.EOF {
//			klog.Errorf("Read error: %v", err)
//			return err
//		}
//
//		if n > 0 {
//			_, err = fileInZip.Write(buf[:n])
//			if err != nil {
//				klog.Errorf("Write error: %v (offset: %d, size: %d, path: %s)", err, bytesRead, n, relPath)
//				return fmt.Errorf("write failed at offset %d: %v", bytesRead, err)
//			}
//			*processedBytes += int64(n)
//			bytesRead += n
//			progress = float64(*processedBytes) * 100 / float64(totalSize)
//
//			if progress-*lastReported >= reportInterval {
//				klog.Infof("Progress: %.2f%% (%s/%s)", progress, formatBytes(*processedBytes), formatBytes(totalSize))
//				*lastReported = progress
//				t.UpdateProgress(int(progress), int64(bytesRead))
//				bytesRead = 0
//			}
//		}
//
//		if err == io.EOF {
//			if bytesRead != 0 {
//				klog.Infof("Progress: %.2f%% (%s/%s)", progress, formatBytes(*processedBytes), formatBytes(totalSize))
//				*lastReported = progress
//				t.UpdateProgress(int(progress), int64(bytesRead))
//				bytesRead = 0
//			}
//			break
//		}
//	}
//
//	if closer, ok := fileInZip.(io.Closer); ok {
//		closer.Close()
//	}
//
//	return nil
//}

func (c *ZipCompressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string,
	totalSize int64, t *TaskFuncs) error {
	// 创建临时文件路径（关键修改1）
	tempPath := outputPath + ".tmp"
	tmpFile, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	defer func() {
		tmpFile.Close()
		if t.GetCompressPaused() {
			os.Remove(outputPath)
			os.Rename(tempPath, outputPath)
		} else {
			os.Remove(tempPath) // 确保清理
		}
	}()

	// 处理原始文件内容
	processedFiles := make(map[string]bool)
	if stat, err := os.Stat(outputPath); err == nil && stat.Size() > 0 {
		// 尝试读取原始ZIP内容
		r, err := zip.NewReader(tmpFile, stat.Size())

		// 只有有效ZIP文件才复制内容
		if err == nil {
			// 复制原始文件内容
			src, _ := os.Open(outputPath)
			defer src.Close()
			io.Copy(tmpFile, src)

			// 重置文件指针
			tmpFile.Seek(0, io.SeekStart)

			// 记录已处理文件
			for _, f := range r.File {
				processedFiles[f.Name] = true
			}
		} else {
			klog.Infof("Warning: %s is not a valid ZIP file, creating new archive", outputPath)
		}
	}

	// 创建ZIP写入器
	zw := zip.NewWriter(tmpFile)
	defer zw.Close()

	// 读取原始文件列表（如果存在）
	// 获取文件信息
	//fileInfo, err := tmpFile.Stat()
	//if err != nil {
	//	return fmt.Errorf("failed to read tmpFile info: %v", err)
	//}
	//
	//// 创建ZIP读取器
	//r, err := zip.NewReader(tmpFile, fileInfo.Size())
	//if err != nil {
	//	return fmt.Errorf("failed to create ZIP reader: %v", err)
	//}
	//processedFiles = make(map[string]bool)
	//for _, f := range r.File {
	//	processedFiles[f.Name] = true
	//}

	klog.Infof("[ZIP running LOG] task: %+v", t)
	resumeIndex, resumeBytes := t.GetCompressPauseInfo()
	klog.Infof("[ZIP running LOG] got pause info: resumeIndex: %d, resumeBytes: %d", resumeIndex, resumeBytes)

	processedBytes := int64(0)
	if resumeBytes != 0 {
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
		if t.GetCompressPaused() {
			klog.Infof("[ZIP running LOG] Paused compressing before starting")
			t.SetCompressPauseInfo(currentFileIndex, processedBytes)
		} else {
			klog.Infof("[ZIP running LOG] Cancelled compressing before starting")
		}
		return ctx.Err()
	default:
	}

	//f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	//if err != nil {
	//	klog.Errorf("Failed to open zip file: %v", err)
	//	return err
	//}
	//defer f.Close()
	//
	//zw := zip.NewWriter(f)
	//defer zw.Close()
	//
	//processedFiles := make(map[string]struct{}, len(relPathList))
	//for i := 0; i < resumeIndex; i++ {
	//	processedFiles[relPathList[i]] = struct{}{}
	//}

	for index := currentFileIndex; index < len(fileList); index++ {
		if processedFiles[relPathList[index]] {
			continue // 跳过已处理文件
		}

		relPath := relPathList[index]
		filePath := fileList[index]

		select {
		case <-ctx.Done():
			if t.GetCompressPaused() {
				klog.Infof("[ZIP running LOG] Paused compressing file: %s", filepath.Base(filePath))
				t.SetCompressPauseInfo(index, processedBytes)
			} else {
				klog.Infof("[ZIP running LOG] Cancelled compressing file: %s", filepath.Base(filePath))
				err = os.RemoveAll(outputPath)
				if err != nil {
					klog.Errorf("[ZIP running LOG] Failed to remove file: %v", err)
				}
			}
			return ctx.Err()
		default:
		}

		//info, err := os.Stat(filePath)
		//if err != nil {
		//	klog.Errorf("stat error: %v", err)
		//	continue
		//}

		//if info.IsDir() {
		//	if !strings.HasSuffix(relPath, "/") {
		//		relPath += "/"
		//	}
		//	_, err = zw.Create(relPath)
		//	if err != nil {
		//		klog.Errorf("Failed to create directory entry: %v", err)
		//		return err
		//	}
		//	processedBytes += 4096
		//	progress := float64(processedBytes) * 100 / float64(totalSize)
		//	klog.Infof("Progress: %.2f%% (Directory: %s)", progress, relPath)
		//	lastReported = progress
		//	t.UpdateProgress(int(progress), 4096)
		//	t.SetCompressPauseInfo(index, processedBytes)
		//	continue
		//}
		//
		//// 关键修改2：使用正确的CreateHeader方法
		//header, _ := zip.FileInfoHeader(info)
		//header.Name = relPath
		//header.Method = zip.Deflate
		//
		//// 关键修改3：使用CreateHeader创建文件头
		//fileInZip, err := zw.CreateHeader(header)
		//if err != nil {
		//	klog.Errorf("CreateHeader failed: %v", err)
		//	continue
		//}

		// 关键修改4：传递文件头信息到写入函数
		err = addFileToZip(zw, filePath, relPath,
			totalSize, &processedBytes, &lastReported, reportInterval, t)
		if err != nil {
			klog.Errorf("Add file failed: %v", err)
			return err
		}

		t.SetCompressPauseInfo(index, processedBytes)
		klog.Infof("[ZIP running LOG] index: %d, processedBytes: %d", index, processedBytes)
		time.Sleep(5 * time.Second)
	}

	// 原子替换（关键修改4）
	zw.Close()
	tmpFile.Close()
	os.Remove(outputPath)
	os.Rename(tempPath, outputPath)

	if totalSize > 0 {
		progress := 100.0
		klog.Infof("Compression completed: %.2f%%", progress)
		t.UpdateProgress(int(progress), 0)
	}
	return nil
}

// 完全保留原始控制逻辑的addFileToZip函数
func addFileToZip(
	zw *zip.Writer,
	srcPath, relPath string,
	totalSize int64,
	processedBytes *int64,
	lastReported *float64,
	reportInterval float64,
	t *TaskFuncs,
) error {

	// 完全保留原始文件处理逻辑
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if !strings.HasSuffix(relPath, "/") {
			relPath += "/"
		}
		_, err := zw.Create(relPath)
		if err != nil {
			return err
		}
		*processedBytes += 4096
		progress := float64(*processedBytes) * 100 / float64(totalSize)
		*lastReported = progress
		t.UpdateProgress(int(progress), 4096)
		return nil
	}

	header, _ := zip.FileInfoHeader(info)
	header.Name = relPath
	header.Method = zip.Deflate

	fileInZip, _ := zw.Create(header.Name)
	srcFile, _ := os.Open(srcPath)
	defer srcFile.Close()

	buf := make([]byte, 4096)
	bytesRead := 0
	progress := float64(0)

	for {
		n, err := srcFile.Read(buf)
		if err != nil && err != io.EOF {
			klog.Errorf("Read error: %v", err)
			return err
		}

		if n > 0 {
			_, err = fileInZip.Write(buf[:n])
			if err != nil {
				klog.Errorf("Write error: %v", err)
				return fmt.Errorf("write failed: %v", err)
			}
			*processedBytes += int64(n)
			bytesRead += n
			progress = float64(*processedBytes) * 100 / float64(totalSize)

			if progress-*lastReported >= reportInterval {
				klog.Infof("Progress: %.2f%% (%s/%s)", progress, formatBytes(*processedBytes), formatBytes(totalSize))
				*lastReported = progress
				t.UpdateProgress(int(progress), int64(bytesRead))
				bytesRead = 0
			}
		}

		if err == io.EOF {
			if bytesRead != 0 {
				klog.Infof("Progress: %.2f%% (%s/%s)", progress, formatBytes(*processedBytes), formatBytes(totalSize))
				*lastReported = progress
				t.UpdateProgress(int(progress), int64(bytesRead))
				bytesRead = 0
			}
			break
		}
	}

	// 关键修改12：显式关闭文件写入器
	//if closer, ok := fileInZip.(io.Closer); ok {
	//	closer.Close()
	//}

	return nil
}

// ZIP解压
//func (c *ZipCompressor) Uncompress(
//	ctx context.Context,
//	src, dest string,
//	override bool,
//	callbackup func(p int, t int64),
//) error {
//	r, err := zip.OpenReader(src)
//	if err != nil {
//		return err
//	}
//	defer r.Close()
//
//	total := len(r.File)
//	processed := 0
//
//	for _, f := range r.File {
//		select {
//		case <-ctx.Done():
//			klog.Infof("[ZIP running LOG] Cancelling uncompressed file: %s", f.Name)
//			err = os.RemoveAll(dest)
//			if err != nil {
//				klog.Errorf("[ZIP running LOG] Failed to remove file: %v", err)
//			}
//			return ctx.Err()
//		default:
//		}
//
//		fpath := filepath.Join(dest, f.Name)
//		if !strings.HasPrefix(fpath, dest+"/") {
//			return fmt.Errorf("非法路径: %s", f.Name)
//		}
//
//		if f.FileInfo().IsDir() {
//			os.MkdirAll(fpath, 0755)
//			processed++
//			klog.Infof("进度: %d/%d (%.2f%%) - %s",
//				processed, total,
//				float64(processed)/float64(total)*100,
//				f.Name)
//			callbackup(int(float64(processed)/float64(total)*100), 0)
//			continue
//		}
//
//		if !override {
//			if _, err := os.Stat(fpath); err == nil {
//				klog.Infof("跳过已存在的文件: %s", fpath)
//				processed++
//				klog.Infof("进度: %d/%d (%.2f%%)",
//					processed, total,
//					float64(processed)/float64(total)*100)
//				callbackup(int(float64(processed)/float64(total)*100), 0)
//				continue
//			}
//		}
//
//		os.MkdirAll(filepath.Dir(fpath), 0755)
//
//		out, err := os.Create(fpath)
//		if err != nil {
//			return err
//		}
//
//		rc, err := f.Open()
//		if err != nil {
//			out.Close()
//			return err
//		}
//
//		_, err = io.Copy(out, rc)
//		out.Close()
//		rc.Close()
//
//		if err != nil {
//			return err
//		}
//
//		processed++
//		klog.Infof("进度: %d/%d (%.2f%%) - %s",
//			processed, total,
//			float64(processed)/float64(total)*100,
//			f.Name)
//		callbackup(int(float64(processed)/float64(total)*100), 0)
//	}
//	return nil
//}

func (c *ZipCompressor) Uncompress(
	ctx context.Context,
	src, dest string,
	override bool,
	//callbackup func(p int, t int64),
	t *TaskFuncs, // 新增任务控制参数
) error {
	// 获取暂停恢复点
	resumeIndex, resumeBytes := t.GetCompressPauseInfo()
	klog.Infof("[ZIP running LOG] Uncompress resume: index=%d, bytes=%d", resumeIndex, resumeBytes)

	// 初始化进度跟踪
	processedBytes := int64(0)
	currentFileIndex := 0
	if resumeBytes != 0 {
		processedBytes = resumeBytes
	}
	if resumeIndex != 0 {
		currentFileIndex = resumeIndex
	}

	// 打开ZIP文件
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// 预计算文件尺寸和总大小
	fileSizes := make([]int64, len(r.File))
	totalSize := int64(0)
	for i, f := range r.File {
		size := f.UncompressedSize64
		fileSizes[i] = int64(size)
		totalSize += int64(size)
	}

	processedFiles := 0
	//lastReport := time.Now()
	//reportInterval := 500 * time.Millisecond

	// 上下文取消检查
	select {
	case <-ctx.Done():
		if t.GetCompressPaused() {
			t.SetCompressPauseInfo(currentFileIndex, processedBytes)
			return ctx.Err()
		}
		os.RemoveAll(dest)
		return ctx.Err()
	default:
	}

	for index := currentFileIndex; index < len(r.File); index++ {
		f := r.File[index]
		fpath := filepath.Join(dest, f.Name)

		// 上下文检查
		select {
		case <-ctx.Done():
			if t.GetCompressPaused() {
				t.SetCompressPauseInfo(index, processedBytes)
				return ctx.Err()
			}
			os.RemoveAll(dest)
			return ctx.Err()
		default:
		}

		// 路径安全校验
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(filepath.Separator)) {
			return fmt.Errorf("非法路径: %s", f.Name)
		}

		// 处理目录
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			processedFiles++
			klog.Infof("进度: %d/%d (%.2f%%) - %s",
				processedFiles, len(r.File),
				float64(processedFiles)/float64(len(r.File))*100,
				f.Name)
			t.UpdateProgress(int(float64(processedFiles)/float64(len(r.File))*100), processedBytes)
			t.SetCompressPauseInfo(index+1, processedBytes)
			continue
		}

		// 文件存在性检查
		if !override {
			if _, err := os.Stat(fpath); err == nil {
				klog.Infof("跳过已存在文件: %s", fpath)
				processedFiles++
				klog.Infof("进度: %d/%d (%.2f%%)",
					processedFiles, len(r.File),
					float64(processedFiles)/float64(len(r.File))*100)
				t.UpdateProgress(int(float64(processedFiles)/float64(len(r.File))*100), processedBytes)
				t.SetCompressPauseInfo(index+1, processedBytes)
				continue
			}
		}

		// 创建目标目录
		os.MkdirAll(filepath.Dir(fpath), 0755)

		// 打开源文件
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		// ★★★ 关键修改1：恢复点处理（零Seek实现）
		var reader io.Reader = rc
		if index == currentFileIndex && resumeBytes > 0 {
			prevTotal := int64(0)
			for i := 0; i < currentFileIndex; i++ {
				prevTotal += fileSizes[i]
			}
			offset := resumeBytes - prevTotal
			if offset < 0 {
				offset = 0
			}
			// 使用LimitReader实现断点续传
			reader = io.LimitReader(rc, offset)
		}

		// ★★★ 关键修改2：内存缓冲区桥接
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, reader)
		if err != nil {
			return err
		}

		// 打开目标文件
		out, err := os.Create(fpath)
		if err != nil {
			return err
		}
		defer out.Close()

		// ★★★ 关键修改3：写入数据（零Seek实现）
		_, err = out.Write(buf.Bytes())
		if err != nil {
			return err
		}

		// 更新进度
		processedBytes += fileSizes[index]
		processedFiles++

		// 进度报告
		progress := float64(processedBytes) / float64(totalSize) * 100
		t.UpdateProgress(int(progress), processedBytes)
		t.SetCompressPauseInfo(index+1, processedBytes)

		klog.Infof("解压完成: %s (大小: %d字节)", f.Name, fileSizes[index])
	}

	// 最终完成处理
	t.UpdateProgress(100, processedBytes)
	klog.Infof("解压任务完成，总处理量: %d字节", processedBytes)
	return nil
}
