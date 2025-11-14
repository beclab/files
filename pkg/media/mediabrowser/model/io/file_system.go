package io

import (
	"time"
)

type IFileSystem interface {
	/*
	   IsShortcut(filename string) bool
	   ResolveShortcut(filename string) (string, error)
	   CreateShortcut(shortcutPath, target string)
	   MakeAbsolutePath(folderPath, filePath string) string
	   GetFileSystemInfo(path string) (FileSystemMetadata, error)
	   GetFileInfo(path string) (FileSystemMetadata, error)
	   GetDirectoryInfo(path string) (FileSystemMetadata, error)
	   GetValidFilename(filename string) string
	   GetCreationTimeUtc2(info FileSystemMetadata) time.Time
	   GetCreationTimeUtc(path string) (time.Time, error)
	   GetLastWriteTimeUtc(info FileSystemMetadata) time.Time
	*/

	GetLastWriteTimeUtc(path string) (time.Time, error)
	/*
	   SwapFiles(file1, file2 string)
	   AreEqual(path1, path2 string) bool
	   ContainsSubPath(parentPath, path string) bool
	   GetFileNameWithoutExtension(info FileSystemMetadata) string
	   IsPathFile(path string) bool
	*/
	DeleteFile(path string) error
	/*
	   GetDirectories(path string, recursive bool) ([]FileSystemMetadata, error)
	*/
	GetFiles(path string, recursive bool) ([]FileSystemMetadata, error)
	GetFilesWithFilter(path string, extensions []string, enableCaseSensitiveExtensions bool, recursive bool) ([]FileSystemMetadata, error)
	/*
	   GetFileSystemEntries(path string, recursive bool) ([]FileSystemMetadata, error)
	   GetDirectoryPaths(path string, recursive bool) ([]string, error)
	*/
	GetFilePaths(path string, recursive bool) ([]string, error)
	/*
	   GetFilePathsWithFilter(path string, extensions []string, enableCaseSensitiveExtensions bool, recursive bool) ([]string, error)
	   GetFileSystemEntryPaths(path string, recursive bool) ([]string, error)
	   SetHidden(path string, isHidden bool)
	*/
	SetAttributes(path string, isHidden, readOnly bool) error
	/*
	   GetDrives() ([]FileSystemMetadata, error)
	*/
	DirectoryExists(path string) bool
	FileExists(path string) bool
}
