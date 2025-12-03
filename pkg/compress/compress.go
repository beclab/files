package compress

import (
	"context"
	"fmt"
	"github.com/h2non/filetype"
	"io"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// 压缩格式类型
const (
	FormatZIP     = "zip"
	FormatRAR     = "rar"
	Format7Z      = "7z"
	FormatTAR     = "tar"
	FormatGZIP    = "gzip"
	FormatTARGZ   = "tar.gz"
	FormatBZ2     = "bz2"
	FormatTARBZ2  = "tar.bz2"
	FormatXZ      = "xz"
	FormatTARXZ   = "tar.xz"
	FormatUnknown = "unknown"
)

type TaskFuncs struct {
	UpdateProgress       func(progress int, transfer int64)
	GetCompressPauseInfo func() (int, int64)
	SetCompressPauseInfo func(index int, bytes int64)
	GetCompressPaused    func() bool
}

// 统一压缩接口（同步版本）
type Compressor interface {
	Compress(ctx context.Context, outputPath string, fileList, relPathList []string, totalSize int64, t *TaskFuncs) error
	Uncompress(ctx context.Context, src, dest string, override bool, t *TaskFuncs) error
}

// 工厂函数扩展
func GetCompressor(format string) (Compressor, error) {
	switch format {
	case FormatZIP:
		return &ZipCompressor{}, nil
	//case FormatTAR:
	//	return &TarCompressor{}, nil
	//case FormatTARGZ, "tgz":
	//	return &TarGzipCompressor{}, nil
	//case FormatTARBZ2, "tbz2":
	//	return &TarBzip2Compressor{}, nil
	//case FormatTARXZ, "txz":
	//	return &TarXzCompressor{}, nil
	//case FormatRAR:
	//	binPath := findRarBin()
	//	if binPath == "" {
	//		return nil, fmt.Errorf("RAR executable not found")
	//	}
	//	return &RarCompressor{binPath: binPath}, nil
	//case Format7Z:
	//	binPath := find7zBin()
	//	if binPath == "" {
	//		return nil, fmt.Errorf("7z executable not found")
	//	}
	//	return &SevenZipCompressor{binPath: binPath}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// 检测解压缩格式
func DetectCompressionType(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return FormatUnknown, err
	}
	defer f.Close()

	// 增加缓冲区大小
	buf := make([]byte, 2048)
	n, _ := f.Read(buf)
	kind, _ := filetype.Match(buf[:n])

	//// 双重验证机制
	//if isXz := xz.ValidHeader(buf); isXz {
	//	return detectNestedTarXz(buf, f)
	//}

	// 第二步：扩展格式检测
	ext := strings.ToLower(filepath.Ext(filePath))
	switch {
	case kind.MIME.Value == "application/x-gzip" || ext == ".gz":
		return detectNestedTar(f, FormatTARGZ)
	case kind.MIME.Value == "application/x-bzip2" || ext == ".bz2":
		return detectNestedTar(f, FormatTARBZ2)
	case kind.MIME.Value == "application/x-xz" || ext == ".xz":
		return detectNestedTar(f, FormatTARXZ)
	case kind.MIME.Value == "application/x-tar" || ext == ".tar":
		return FormatTAR, nil
	case kind.MIME.Value == "application/zip" || ext == ".zip":
		return FormatZIP, nil
	case kind.MIME.Value == "application/x-rar-compressed" || ext == ".rar":
		return FormatRAR, nil
	case kind.MIME.Value == "application/x-7z-compressed" || ext == ".7z":
		return Format7Z, nil
	default:
		return "", fmt.Errorf("不支持的压缩格式: %s", kind.Extension)
	}
}

// 进度报告判断函数
func shouldReport(current, last, interval float64) bool {
	diff := current - last
	return diff >= interval || (current >= 100 && last < 100)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// 缓冲区大小智能配置
func bufferSize(fileSize int64) int {
	baseSize := 32 * 1024 // 默认32KB
	if fileSize > 100*1024*1024 {
		return 1 * 1024 * 1024 // 大文件用1MB缓冲区
	}
	if fileSize > 10*1024*1024 {
		return 512 * 1024 // 中等文件用512KB
	}
	return baseSize
}

// 可执行文件验证（Linux/macOS）
func isExecutable(path string) bool {
	// 1. 存在性检查
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// 2. 类型检查：必须是常规文件（非目录）
	if info.IsDir() {
		return false
	}

	// 3. 权限检查（Unix权限模式）
	mode := info.Mode()
	switch runtime.GOOS {
	case "darwin", "linux": // macOS和Linux
		// 检查所有者/组/其他用户的执行权限
		return mode&0111 != 0
	default:
		// 其他系统默认返回false
		return false
	}
}

// 路径去重
func deduplicatePaths(paths []string) []string {
	set := make(map[string]bool)
	result := []string{}
	for _, p := range paths {
		normalized := filepath.Clean(p)
		if !set[normalized] {
			set[normalized] = true
			result = append(result, normalized)
		}
	}
	return result
}

// 带上下文的复制函数
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) error {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := src.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if _, err := dst.Write(buf[:n]); err != nil {
			return err
		}
	}
}

// 修复进度超过100%的问题
func copyWithProgress(ctx context.Context, dstDir string, dst io.Writer, src io.Reader, processedSize *int64, totalSize int64,
	lastProgress *float64, progressStart float64, name string,
	startTime time.Time, callbackup func(p int, t int64)) error {
	//var processedSize int64
	buffer := make([]byte, 1024*1024) // 1MB缓冲区

	for {
		//if ctx.Err() != nil {
		//	return ctx.Err()
		//}
		select {
		case <-ctx.Done():
			klog.Infof("[TAR.XX running LOG] Try to remove file: %s", dstDir)
			err := os.RemoveAll(dstDir)
			if err != nil {
				klog.Errorf("[TAR.XX running LOG] Failed to remove file: %v", err)
			}
			return ctx.Err()
		default:
		}

		n, err := io.ReadFull(src, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}

		if n > 0 {
			w, err := dst.Write(buffer[:n])
			if err != nil {
				return err
			}
			*processedSize += int64(w)

			// 确保processedSize不超过totalSize
			if *processedSize > totalSize {
				*processedSize = totalSize
			}

			// 精确计算进度百分比
			progress := progressStart + float64(*processedSize)/float64(totalSize)*(100-progressStart)
			klog.Infof("[TAR.XX running LOG] processedSize = %d, totalSize = %d, progress = %f", *processedSize, totalSize, progress)

			// 达到报告间隔时更新进度
			//if processedSize%1024*1024 == 0 || processedSize == totalSize {
			if time.Since(startTime).Seconds() > 1 || progress-*lastProgress >= 1.0 {
				klog.Infof("解压进度: %.1f%% (%s)", progress, name)
				callbackup(int(progress), 0)
				*lastProgress = progress
			}
		}

		if err == io.EOF {
			break
		}
	}
	return nil
}
