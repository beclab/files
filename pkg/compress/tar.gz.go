package compress

import (
	"archive/tar"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
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

// TAR.GZ压缩器
type TarGzipCompressor struct{}

func (c *TarGzipCompressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string, totalSize int64, callbackup func(p int, t int64)) error {
	// 创建输出文件
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %v", err)
	}
	defer outFile.Close()

	// 初始化压缩流
	gw := gzip.NewWriter(outFile)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	processed := int64(0)
	lastReported := -1.0
	reportInterval := 0.5

	// 预处理路径：标准化路径格式
	for i := range relPathList {
		// 去除开头的斜杠，确保相对路径
		relPathList[i] = strings.TrimPrefix(relPathList[i], "/")
		// 确保目录以斜杠结尾
		if strings.HasSuffix(relPathList[i], "/") ||
			(len(relPathList[i]) > 0 && !strings.Contains(relPathList[i], ".")) {
			relPathList[i] = strings.TrimRight(relPathList[i], "/") + "/"
		}
	}

	// 创建路径缓存（避免重复处理相同目录）
	pathCache := make(map[string]bool)

	// 遍历处理文件
	for index, filePath := range fileList {
		select {
		case <-ctx.Done():
			klog.Infof("[TAR.GZ running LOG] Cancelled compressing file: %s", filepath.Base(filePath))
			err = os.RemoveAll(outputPath)
			if err != nil {
				klog.Errorf("[TAR.GZ running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		relPath := relPathList[index]
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open file: %v", err)
		}

		info, err := file.Stat()
		if err != nil {
			file.Close()
			return fmt.Errorf("stat file: %v", err)
		}

		// 处理目录类型
		isDir := info.IsDir()
		if isDir {
			// 确保目录路径存在
			relPath = strings.TrimRight(relPath, "/") + "/"
		}

		// 创建tar头信息
		h := &tar.Header{
			Name:     relPath,
			Size:     info.Size(),
			Mode:     int64(info.Mode()),
			ModTime:  info.ModTime(),
			Typeflag: tar.TypeReg, // 默认普通文件
		}

		// 根据文件类型设置类型标志
		if isDir {
			h.Typeflag = tar.TypeDir
			h.Name += "/" // 确保目录以/结尾
		} else if info.Mode()&os.ModeSymlink != 0 {
			h.Typeflag = tar.TypeSymlink
			h.Linkname = file.Name()
		}

		// 写入头信息
		if err := tw.WriteHeader(h); err != nil {
			file.Close()
			return fmt.Errorf("write header: %v", err)
		}

		// 处理空文件（0字节文件）
		if h.Size == 0 && !isDir {
			// 显式写入空文件标记
			if _, err = tw.Write([]byte{}); err != nil {
				file.Close()
				return fmt.Errorf("write empty file: %v", err)
			}
		}

		// 处理非目录文件
		var bytesCopied int64 = 0
		if !isDir {
			// 使用io.Copy避免EOF问题
			bytesCopied, err = io.Copy(tw, file)
			if err != nil {
				file.Close()
				return fmt.Errorf("copy data: %v", err)
			}

			// 更新全局进度
			processed += bytesCopied
		}

		// 更新进度报告
		progress := float64(processed) * 100 / float64(totalSize)
		if shouldReport(progress, lastReported, reportInterval) {
			klog.Infof("TAR.GZ Progress: %.2f%% (%s/%s) - File: %s",
				progress,
				formatBytes(processed),
				formatBytes(totalSize),
				filepath.Base(relPath))
			lastReported = progress
			callbackup(int(progress), bytesCopied)
		}

		file.Close()

		// 缓存已处理的路径
		pathCache[relPath] = true
	}

	// 最终完成报告
	klog.Infof("TAR.GZ Compression Complete: 100%%")
	return nil
}

// 统一嵌套检测逻辑
func detectNestedTar(f *os.File, format string) (string, error) {
	f.Seek(0, io.SeekStart)
	reader := bufio.NewReader(f)

	// 根据格式创建解压器
	unzipper, baseFormat, err := createDecompressor(reader, format)
	if err != nil {
		return baseFormat, nil // 回退到基础格式
	}

	// 验证tar格式
	tarReader := tar.NewReader(unzipper)
	header, err := tarReader.Next()
	if err == nil && header != nil {
		return format, nil // 确认嵌套格式
	}

	return baseFormat, nil // 回退到基础格式
}

// 统一解压器创建
func createDecompressor(r io.Reader, format string) (io.Reader, string, error) {
	switch format {
	case FormatTARGZ:
		gr, err := gzip.NewReader(r)
		return gr, FormatGZIP, err
	case FormatTARBZ2:
		return bzip2.NewReader(r), FormatBZ2, nil
	case FormatTARXZ:
		xr, err := xz.NewReader(r)
		return xr, FormatXZ, err
	default:
		return nil, "", fmt.Errorf("不支持的格式")
	}
}

func (c *TarGzipCompressor) Uncompress(ctx context.Context, src, dest string, override bool, callbackup func(p int, t int64)) error {
	// 提取基础文件名（不含任何扩展名）
	//baseName := baseNameWithoutExt(filepath.Base(src))

	// 构建目标目录路径
	destDir := dest // filepath.Join(filepath.Dir(dest), baseName)

	// 打开源文件
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := fileInfo.Size()

	// 创建gzip解压器
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	// 创建tar解包器
	tarReader := tar.NewReader(gzReader)

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
			klog.Infof("[TAR.GZ running LOG] Cancelled uncompressing before starting")
			klog.Infof("[TAR.GZ running LOG] Try to remove file befor starting: %s", destDir)
			err = os.RemoveAll(destDir)
			if err != nil {
				klog.Errorf("[TAR.GZ running LOG] Failed to remove file: %v", err)
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
	gzReader, err = gzip.NewReader(file)
	if err != nil {
		return err
	}
	tarReader = tar.NewReader(gzReader)

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
			return err
		}

		select {
		case <-ctx.Done():
			klog.Infof("[TAR.GZ running LOG] Cancelled uncompressing file: %s", header.Name)
			klog.Infof("[TAR.GZ running LOG] Try to remove file: %s", destDir)
			err = os.RemoveAll(destDir)
			if err != nil {
				klog.Errorf("[TAR.GZ running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		// 构建目标路径
		targetPath := filepath.Join(destDir, header.Name)

		// 处理目录
		if header.Typeflag == tar.TypeDir {
			if err = os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
			processedSize += header.Size
			processedFiles++
			continue
		}

		// 处理文件
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		// 检查覆盖逻辑
		if _, err := os.Stat(targetPath); err == nil && !override {
			return fmt.Errorf("文件已存在且禁止覆盖: %s", targetPath)
		}

		// 创建目标文件
		out, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer out.Close()

		// 设置文件权限
		if err = os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
			return err
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
