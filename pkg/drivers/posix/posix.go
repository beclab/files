package posix

import (
	"encoding/json"
	"files/pkg/constant"
	"files/pkg/drivers/base"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"files/pkg/preview"
	"files/pkg/utils"
	"fmt"
	"io"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

var (
	Expand    = true
	Content   = true
	NoExpand  = false
	NoContent = false
)

type PosixStorage struct {
	Handler *base.HandlerParam
}

func (s *PosixStorage) List(fileParam *models.FileParam) ([]byte, error) {
	var owner = s.Handler.Owner

	klog.Infof("POSIX list, owner: %s, param: %s", owner, fileParam.Json())

	fileData, err := s.getFiles(fileParam, Expand, Content)
	if err != nil {
		return nil, err
	}

	if s.isExternal(fileParam.FileType, fileParam.Extend) {
		for _, f := range fileData.Items {
			f.ExternalType = global.GlobalMounted.CheckExternalType(f.Name)
		}
	}

	if fileData.IsDir {
		fileData.Listing.Sorting = files.DefaultSorting
		fileData.Listing.ApplySort()
	}

	res, err := json.Marshal(fileData)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *PosixStorage) Preview(fileParam *models.FileParam, queryParam *models.QueryParam) ([]byte, error) {
	var owner = s.Handler.Owner

	klog.Infof("POSIX preview, owner: %s, param: %s, query: %s", owner, fileParam.Json(), queryParam.Json())

	fileData, err := s.getFiles(fileParam, Expand, Content)
	if err != nil {
		return nil, err
	}

	switch fileData.Type {
	case "image":
		return preview.HandleImagePreview(fileData, queryParam)
	default:
		return nil, fmt.Errorf("can't create preview for %s type", fileData.Type)
	}
}

func (s *PosixStorage) Raw(fileParam *models.FileParam, queryParam *models.QueryParam) (io.ReadCloser, map[string]string, error) {
	var owner = s.Handler.Owner

	klog.Infof("POSIX raw, owner: %s, param: %s", owner, fileParam.Json())

	fileData, err := s.getFiles(fileParam, NoExpand, NoContent)
	if err != nil {
		return nil, nil, err
	}

	if fileData.IsDir {
		return nil, nil, fmt.Errorf("not supported currently")
	}

	r, err := getRawFile(fileData)
	if err != nil {
		return nil, nil, err
	}
	return r, nil, nil
}

func (s *PosixStorage) Stream(fileParam *models.FileParam, stopChan chan struct{}, dataChan chan string) error {
	var owner = s.Handler.Owner

	klog.Infof("POSIX stream, owner: %s, param: %s", owner, fileParam.Json())

	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return err
	}

	var fs = afero.NewBasePathFs(afero.NewOsFs(), resourceUri)

	fileData, err := s.getFiles(fileParam, Expand, NoContent)

	go s.generateListingData(fs, fileParam, fileData.Listing, stopChan, dataChan)

	return nil
}

func (s *PosixStorage) generateListingData(fs afero.Fs, fileParam *models.FileParam, listing *files.Listing, stopChan <-chan struct{}, dataChan chan<- string) {
	defer close(dataChan)

	var streamFiles []*files.FileInfo
	streamFiles = append(streamFiles, listing.Items...)

	for len(streamFiles) > 0 {
		firstItem := streamFiles[0]

		if firstItem.IsDir {
			var nestFileParam = &models.FileParam{
				FileType: fileParam.FileType,
				Extend:   fileParam.Extend,
				Path:     firstItem.Path,
			}

			nestFileData, err := s.getFiles(nestFileParam, Expand, NoContent)
			if err != nil {
				klog.Error(err)
				return
			}

			var nestedItems []*files.FileInfo
			if nestFileData.Listing != nil {
				nestedItems = append(nestedItems, nestFileData.Listing.Items...)
			}
			streamFiles = append(nestedItems, streamFiles[1:]...)
		} else {
			dataChan <- fmt.Sprintf("%s\n\n", utils.ToJson(firstItem))
			streamFiles = streamFiles[1:]
		}

		select {
		case <-stopChan:
			return
		default:
		}
	}
}

func (s *PosixStorage) isExternal(fileType string, extend string) bool {
	return (fileType == constant.External || fileType == constant.Usb || fileType == constant.Hdd || fileType == constant.Internal || fileType == constant.Smb) && extend != ""
}

func (s *PosixStorage) getFiles(fileParam *models.FileParam, expand, content bool) (*files.FileInfo, error) {
	var resourceUri, err = fileParam.GetResourceUri()
	if err != nil {
		return nil, err
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:       afero.NewBasePathFs(afero.NewOsFs(), resourceUri),
		FsType:   fileParam.FileType,
		FsExtend: fileParam.Extend,
		Path:     fileParam.Path,
		Expand:   expand,
		Content:  content,
	})
	if err != nil {
		return nil, err
	}

	if file == nil {
		return nil, fmt.Errorf("file %s not exists", fileParam.Path)
	}

	return file, nil

}
