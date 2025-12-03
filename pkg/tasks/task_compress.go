package tasks

import (
	"files/pkg/compress"
	"fmt"
	"k8s.io/klog/v2"
)

func (t *Task) Compress() error {
	compressor, err := compress.GetCompressor(t.compressParam.Format)
	if err != nil {
		return err
	}

	err = compressor.Compress(
		t.ctx,
		t.compressParam.DstPath, t.compressParam.FileList,
		t.compressParam.RelPathList, t.compressParam.TotalSize,
		t.UpdateProgress, t.GetCompressPauseInfo, t.SetCompressPauseInfo, t.GetCompressPaused)
	if err != nil {
		klog.Errorf("compression failed: %v", err)
		return fmt.Errorf("compression failed: %v", err)
	}
	return nil
}

func (t *Task) Uncompress() error {
	format, err := compress.DetectCompressionType(t.compressParam.SrcPath)
	if err != nil {
		klog.Errorf(err.Error())
		return err
	}

	compressor, err := compress.GetCompressor(format)
	if err != nil {
		return err
	}
	err = compressor.Uncompress(
		t.ctx, t.compressParam.SrcPath,
		t.compressParam.DstPath, t.compressParam.Override,
		t.UpdateProgress)
	if err != nil {
		klog.Errorf("uncompression failed: %v", err)
		return fmt.Errorf("uncompression failed: %v", err)
	}
	return nil
}
