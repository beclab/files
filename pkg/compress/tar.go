package compress

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
)

// 纯TAR压缩器（仅归档不压缩）
type TarCompressor struct{}

func (c *TarCompressor) Compress(ctx context.Context, outputPath string, fileList, relPathList []string, totalSize int64, callbackup func(p int, t int64)) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %v", err)
	}
	defer outFile.Close()

	tw := tar.NewWriter(outFile)
	defer tw.Close()

	return c.processFiles(ctx, tw, outputPath, fileList, relPathList, totalSize, callbackup)
}

// 共享的文件处理逻辑
func (c *TarCompressor) processFiles(ctx context.Context, tw *tar.Writer, outputPath string, fileList, relPathList []string, totalSize int64, callbackup func(p int, t int64)) error {
	processed := int64(0)
	lastReported := -1.0
	reportInterval := 0.5

	for index, filePath := range fileList {
		select {
		case <-ctx.Done():
			klog.Infof("[TAR running LOG] Cancelled compressing file: %s", filepath.Base(filePath))
			err := os.RemoveAll(outputPath)
			if err != nil {
				klog.Errorf("[TAR running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		relPath := strings.TrimPrefix(relPathList[index], "/")
		if strings.HasSuffix(relPath, "/") ||
			(!strings.Contains(relPath, ".") && len(relPath) > 0) {
			relPath = strings.TrimRight(relPath, "/") + "/"
		}

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open file: %v", err)
		}

		info, err := file.Stat()
		if err != nil {
			file.Close()
			return fmt.Errorf("stat file: %v", err)
		}

		isDir := info.IsDir()
		if isDir {
			relPath = strings.TrimRight(relPath, "/") + "/"
		}

		h := &tar.Header{
			Name:     relPath,
			Size:     info.Size(),
			Mode:     int64(info.Mode()),
			ModTime:  info.ModTime(),
			Typeflag: tar.TypeReg,
		}

		if isDir {
			h.Typeflag = tar.TypeDir
			h.Name += "/"
		} else if info.Mode()&os.ModeSymlink != 0 {
			h.Typeflag = tar.TypeSymlink
			h.Linkname = file.Name()
		}

		if err := tw.WriteHeader(h); err != nil {
			file.Close()
			return fmt.Errorf("write header: %v", err)
		}

		if h.Size == 0 && !isDir {
			if _, err = tw.Write([]byte{}); err != nil {
				file.Close()
				return fmt.Errorf("write empty file: %v", err)
			}
		}

		var bytesCopied int64 = 0
		if !isDir {
			bytesCopied, err = io.Copy(tw, file)
			if err != nil {
				file.Close()
				return fmt.Errorf("copy data: %v", err)
			}
			processed += bytesCopied
		}

		progress := float64(processed) * 100 / float64(totalSize)
		if shouldReport(progress, lastReported, reportInterval) {
			klog.Infof("%s Progress: %.2f%% (%s/%s) - File: %s",
				c.CompressionType(),
				progress,
				formatBytes(processed),
				formatBytes(totalSize),
				filepath.Base(relPath))
			lastReported = progress
			callbackup(int(progress), bytesCopied)
		}

		file.Close()
	}

	klog.Infof("%s Compression Complete: 100%%", c.CompressionType())
	return nil
}

func (c *TarCompressor) CompressionType() string {
	return "TAR"
}

// TAR解压
func (c *TarCompressor) Uncompress(
	ctx context.Context,
	src, dest string,
	override bool,
	callbackup func(p int, t int64),
) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	tarReader := tar.NewReader(file)
	total := 0

	// 第一次遍历计算总数
	tempReader := tar.NewReader(file)
	for {
		_, err := tempReader.Next()
		if err == io.EOF {
			break
		}
		total++
	}

	// 重置文件指针
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}
	tarReader = tar.NewReader(file)

	processed := 0
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
			klog.Infof("[TAR running LOG] Cancelled uncompressing file: %s", header.Name)
			err = os.RemoveAll(dest)
			if err != nil {
				klog.Errorf("[TAR running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		fpath := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(fpath, dest+"/") {
			return fmt.Errorf("非法路径: %s", header.Name)
		}

		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(fpath, 0755)
			processed++
			klog.Infof("进度: %d/%d (%.2f%%) - %s",
				processed, total,
				float64(processed)/float64(total)*100,
				header.Name)
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

		_, err = io.Copy(out, tarReader)
		out.Close()
		if err != nil {
			return err
		}

		processed++
		klog.Infof("进度: %d/%d (%.2f%%) - %s",
			processed, total,
			float64(processed)/float64(total)*100,
			header.Name)
		callbackup(int(float64(processed)/float64(total)*100), 0)
	}
	return nil
}
