package compress

import (
	"archive/tar"
	"compress/bzip2"
	"context"
	"fmt"
	bz2 "github.com/dsnet/compress/bzip2"
	"io"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// tar.bz2压缩器
type TarBzip2Compressor struct{}

func (c *TarBzip2Compressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string, totalSize int64, callbackup func(p int, t int64)) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %v", err)
	}
	defer outFile.Close()

	wc := &bz2.WriterConfig{}
	wc.Level = bz2.BestCompression
	bw, err := bz2.NewWriter(outFile, wc)
	if err != nil {
		return fmt.Errorf("create output: %v", err)
	}
	defer bw.Close()

	tw := tar.NewWriter(bw)
	defer tw.Close()

	return c.processFiles(ctx, tw, outputPath, fileList, relPathList, totalSize, callbackup)
}

func (c *TarBzip2Compressor) CompressionType() string {
	return "TAR.BZ2"
}

func (c *TarBzip2Compressor) processFiles(ctx context.Context, tw *tar.Writer, outputPath string, fileList, relPathList []string, totalSize int64, callbackup func(p int, t int64)) error {
	return (*TarCompressor)(nil).processFiles(ctx, tw, outputPath, fileList, relPathList, totalSize, callbackup)
}

func (c *TarBzip2Compressor) Uncompress(ctx context.Context, src, dest string, override bool, callbackup func(p int, t int64)) error {
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

	// 创建bzip2解压器
	bz2Reader := bzip2.NewReader(file)

	// 创建tar解包器
	tarReader := tar.NewReader(bz2Reader)

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
			klog.Infof("[TAR.BZ2 running LOG] Cancelled uncompressing before starting")
			klog.Infof("[TAR.BZ2 running LOG] Try to remove file befor starting: %s", destDir)
			err = os.RemoveAll(destDir)
			if err != nil {
				klog.Errorf("[TAR.BZ2 running LOG] Failed to remove file: %v", err)
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
		klog.Infof("[TAR.BZ2 running LOG] meta data header.name=%s, totalFiles=%d, totalSize=%d, progress=%f", header.Name, totalFiles, totalSize, progress)
	}
	progress = 50.0

	klog.Infof("开始解压 %s (%d 个文件, 总大小 %s)",
		src, totalFiles, formatBytes(totalSize))

	// 重置tarReader
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	bz2Reader = bzip2.NewReader(file)
	tarReader = tar.NewReader(bz2Reader)

	// 进度跟踪变量
	var processedFiles int
	var processedSize int64 = 0

	// 第二遍遍历：实际解压
	for i, header := range files {
		select {
		case <-ctx.Done():
			klog.Infof("[TAR.BZ2 running LOG] Cancelled uncompressing file: %s", header.Name)
			klog.Infof("[TAR.BZ2 running LOG] Try to remove file: %s", destDir)
			err = os.RemoveAll(destDir)
			if err != nil {
				klog.Errorf("[TAR.BZ2 running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		// 构建目标路径（基于解压目录）
		klog.Infof("destDir=%s", destDir)
		klog.Infof("header.Name=%s", header.Name)
		targetPath := filepath.Join(destDir, strings.TrimPrefix(header.Name, "tar/"))
		klog.Infof("targetPath=%s", targetPath)

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
		if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		// 检查覆盖逻辑
		if _, err = os.Stat(targetPath); err == nil && !override {
			return fmt.Errorf("文件已存在且禁止覆盖: %s", targetPath)
		}

		// 创建目标文件
		out, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer out.Close()

		// 设置文件权限
		if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
			return err
		}

		// 记录开始时间
		//startTime := time.Now()

		// 重置tarReader到当前文件位置
		tarReader, err = seekToPosition(ctx, file, files[:i])
		if err != nil {
			return err
		}

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

// 修复死循环问题：添加上下文取消检查
func seekToPosition(ctx context.Context, file *os.File, skipHeaders []*tar.Header) (*tar.Reader, error) {
	// 重置文件指针
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	// 重新创建bzip2解压器
	newBz2Reader := bzip2.NewReader(file)
	newTarReader := tar.NewReader(newBz2Reader)

	// 跳过已处理的header
	for _, _ = range skipHeaders {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		_, err = newTarReader.Next()
		if err != nil {
			return nil, err
		}
	}

	return newTarReader, nil
}

// 修复文件夹命名问题：完全去除所有扩展名
func baseNameWithoutExt(filename string) string {
	// 定义所有需要去除的扩展名
	extensions := []string{
		".tar.bz2", ".tbz2", ".bz2",
		".tar.gz", ".tgz", ".gz",
		".tar.xz", ".txz", ".xz",
		".tar", ".zip", ".rar", ".7z",
	}

	// 尝试去除所有可能的扩展名
	base := filename
	for _, ext := range extensions {
		if strings.HasSuffix(base, ext) {
			base = strings.TrimSuffix(base, ext)
		}
	}

	// 去除残留的点号和路径分隔符
	base = strings.Trim(base, ".")
	base = filepath.Base(base)

	// 处理双重扩展名情况
	if strings.Contains(base, ".") {
		lastDot := strings.LastIndex(base, ".")
		base = base[:lastDot]
	}

	return base
}
