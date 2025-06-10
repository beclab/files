package models

type ListParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

type DeleteParam struct {
	Path  string `json:"path"`
	Drive string `json:"drive"`
	Name  string `json:"name"`
}

type PatchParam struct {
	Path        string `json:"path"`
	NewFileName string `json:"new_file_name"`
	Drive       string `json:"drive"`
	Name        string `json:"name"`
}

type PostParam struct {
	ParentPath string `json:"parent_path"`
	FolderName string `json:"folder_name"`
	Drive      string `json:"drive"`
	Name       string `json:"name"`
}

type CopyFileParam struct {
	CloudFilePath     string `json:"cloud_file_path"`
	NewCloudDirectory string `json:"new_cloud_directory"`
	NewCloudFileName  string `json:"new_cloud_file_name"`
	Drive             string `json:"drive"`
	Name              string `json:"name"`
}

type MoveFileParam struct {
	CloudFilePath     string `json:"cloud_file_path"`
	NewCloudDirectory string `json:"new_cloud_directory"`
	Drive             string `json:"drive"`
	Name              string `json:"name"`
}

type DownloadAsyncParam struct {
	LocalFolder   string `json:"local_folder"`
	CloudFilePath string `json:"cloud_file_path"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
	LocalFileName string `json:"local_file_name,omitempty"`
}

type UploadAsyncParam struct {
	ParentPath    string `json:"parent_path"`
	LocalFilePath string `json:"local_file_path"`
	NewFileName   string `json:"new_file_name,omitempty"`
	Drive         string `json:"drive"`
	Name          string `json:"name"`
}

type QueryTaskParam struct {
	TaskIds []string `json:"task_ids"`
}
