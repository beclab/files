package compress

import (
	"archive/tar"
	"context"
	"fmt"
	"github.com/ulikunitz/xz"
	"io"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TAR.XZ压缩器
type TarXzCompressor struct{}

func (c *TarXzCompressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string,
	totalSize int64, callbackup func(p int, t int64), resumeIndex *int, resumBytes *int64, paused *bool) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %v", err)
	}
	defer outFile.Close()

	xw, err := xz.NewWriter(outFile)
	if err != nil {
		return fmt.Errorf("create xz writer: %v", err)
	}
	defer xw.Close()

	tw := tar.NewWriter(xw)
	defer tw.Close()

	processed := int64(0) // 已处理字节数
	lastReported := -1.0  // 上次报告的进度
	reportInterval := 0.5 // 报告间隔(百分比)

	for i, filePath := range fileList {
		select {
		case <-ctx.Done():
			klog.Infof("[TAR.XZ running LOG] Cancelled compressing file: %s", filepath.Base(filePath))
			err = os.RemoveAll(outputPath)
			if err != nil {
				klog.Errorf("[TAR.XZ running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		relPath := relPathList[i]
		relPath = strings.TrimPrefix(relPath, "/")
		if strings.HasSuffix(relPath, "/") || (len(relPath) > 0 && !strings.Contains(relPath, ".")) {
			relPath += "/"
		}

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open file: %v", err)
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("stat file: %v", err)
		}

		h := &tar.Header{
			Name:     relPath,
			Size:     info.Size(),
			Mode:     int64(info.Mode()),
			ModTime:  info.ModTime(),
			Typeflag: tar.TypeReg,
		}

		if info.IsDir() {
			h.Typeflag = tar.TypeDir
			h.Name += "/"
		}

		err = tw.WriteHeader(h)
		if err != nil {
			return fmt.Errorf("write header: %v", err)
		}

		var bytesCopied int64 = 0
		if !info.IsDir() {
			// 复制文件内容并更新进度
			bytesCopied, err = io.Copy(tw, file)
			if err != nil {
				return fmt.Errorf("copy data: %v", err)
			}
			processed += bytesCopied
		}

		// 进度报告逻辑
		progress := float64(processed) * 100 / float64(totalSize)
		if shouldReport(progress, lastReported, reportInterval) {
			klog.Infof("TAR.XZ Progress: %.2f%% (%s/%s) - File: %s",
				progress,
				formatBytes(processed),
				formatBytes(totalSize),
				filepath.Base(relPath))
			lastReported = progress
			callbackup(int(progress), bytesCopied)
		}
	}

	// 最终完成报告
	klog.Infof("TAR.XZ Compression Complete: 100%%")
	return nil
}

func (c *TarXzCompressor) Uncompress(ctx context.Context, src, dest string, override bool, callbackup func(p int, t int64)) error {
	// 提取基础文件名（不含任何扩展名）
	//baseName := baseNameWithoutExt(filepath.Base(src))

	// 构建目标目录路径
	destDir := dest // filepath.Join(filepath.Dir(dest), baseName)

	// 打开源文件
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := fileInfo.Size()

	// 创建xz解压器
	xzReader, err := xz.NewReader(file)
	if err != nil {
		return fmt.Errorf("create xz reader: %v", err)
	}

	// 创建tar解压器
	tarReader := tar.NewReader(xzReader)

	// 获取总文件数和总大小（用于进度计算）
	var totalFiles int
	var totalSize int64
	files := make([]*tar.Header, 0)
	var progress float64 = 0.0
	var lastProgress float64 = 0.0

	// 第一遍遍历：获取元数据
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			klog.Infof("[TAR.XZ running LOG] Cancelled uncompressing before starting")
			klog.Infof("[TAR.XZ running LOG] Try to remove file befor starting: %s", destDir)
			err = os.RemoveAll(destDir)
			if err != nil {
				klog.Errorf("[TAR.XZ running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		files = append(files, header)
		totalFiles++
		totalSize += header.Size
		progress = float64(totalSize) / float64(fileSize) * 50
		if progress > 50.0 {
			progress = 50
		}
		if progress-lastProgress >= 1.0 {
			klog.Infof("解析进度: %.1f%% (%s)", progress, header.Name)
			callbackup(int(progress), 0)
			lastProgress = progress
		}
		klog.Infof("[TAR.XZ running LOG] meta data header.name=%s, totalFiles=%d, totalSize=%d, progress=%f", header.Name, totalFiles, totalSize, progress)
	}
	progress = 50.0

	klog.Infof("开始解压 %s (%d 个文件, 总大小 %s)",
		src, totalFiles, formatBytes(totalSize))

	// 重置tarReader
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	xzReader, err = xz.NewReader(file)
	if err != nil {
		return fmt.Errorf("create xz reader: %v", err)
	}
	tarReader = tar.NewReader(xzReader)

	// 进度跟踪变量
	var processedFiles int
	var processedSize int64

	// 遍历tar条目
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %v", err)
		}

		select {
		case <-ctx.Done():
			klog.Infof("[TAR.XZ running LOG] Cancelled uncompressing file: %s", header.Name)
			klog.Infof("[TAR.XZ running LOG] Try to remove file: %s", destDir)
			err = os.RemoveAll(destDir)
			if err != nil {
				klog.Errorf("[TAR.XZ running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		// 构建目标路径
		targetPath := filepath.Join(destDir, header.Name)

		// 处理目录
		if header.Typeflag == tar.TypeDir {
			err = os.MkdirAll(targetPath, os.ModePerm)
			if err != nil {
				return fmt.Errorf("create dir: %v", err)
			}
			processedSize += header.Size
			processedFiles++
			continue
		}

		// 处理文件
		err = os.MkdirAll(filepath.Dir(targetPath), os.ModePerm)
		if err != nil {
			return fmt.Errorf("create dir: %v", err)
		}

		out, err := os.Create(targetPath)
		if err != nil {
			return fmt.Errorf("create file: %v", err)
		}

		// 保留文件权限
		err = os.Chmod(targetPath, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("set permissions: %v", err)
		}

		// 记录开始时间
		//startTime := time.Now()

		// 带进度和上下文的复制
		err = copyWithProgress(ctx, destDir, out, tarReader, &processedSize, totalSize, &progress, 50.0, header.Name, time.Now(), callbackup)
		if err != nil {
			klog.Infof("[TAR.BZ2 running LOG] Try to remove file befor starting: %s", destDir)
			subErr := os.RemoveAll(destDir)
			if subErr != nil {
				klog.Errorf("[TAR.BZ2 running LOG] Failed to remove file: %v", subErr)
			}
			return err
		}

		// 更新进度
		processedFiles++
		processedSize += header.Size
	}

	klog.Infof("解压完成: %s → %s (成功处理 %d 个文件, 总大小 %s)",
		filepath.Base(src), destDir, processedFiles, formatBytes(processedSize))
	return nil
}
