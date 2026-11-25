package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeSSTable
)

type FileMetadata struct {
	fileNum  int
	fileType FileType
}

func (f *FileMetadata) IsSSTable() bool {
	return f.fileType == FileTypeSSTable
}

func (f *FileMetadata) FileNum() int {
	return f.fileNum
}

type Provider struct {
	dataDir string
	fileNum int
}

func NewProvider(dataDir string) (*Provider, error) {
	s := &Provider{dataDir: dataDir}

	err := s.ensureDataDirExists()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Provider) ensureDataDirExists() error {
	err := os.MkdirAll(s.dataDir, 0o700)
	if err != nil {
		return err
	}
	return nil
}

func (s *Provider) ListFiles() ([]*FileMetadata, error) {
	files, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, err
	}

	var meta []*FileMetadata
	var fileNumber int
	var fileExtension string
	for _, f := range files {
		_, err := fmt.Sscanf(f.Name(), "%06d.%s", &fileNumber, &fileExtension)
		if err != nil {
			return nil, err
		}
		fileType := FileTypeUnknown
		if fileExtension == "sst" {
			fileType = FileTypeSSTable
		}
		meta = append(meta, &FileMetadata{
			fileNum:  fileNumber,
			fileType: fileType,
		})
	}
	return meta, nil
}

func (s *Provider) nextFileNum() int {
	s.fileNum++
	return s.fileNum
}

func (s *Provider) makeFileName(fileNumber int) string {
	return fmt.Sprintf("%06d.ssd", fileNumber)
}

func (s *Provider) PrepareNewFile() *FileMetadata {
	return &FileMetadata{
		fileNum:  s.nextFileNum(),
		fileType: FileTypeSSTable,
	}
}

func (s *Provider) OpenFileForWriting(meta *FileMetadata) (*os.File, error) {
	filename := s.makeFileName(meta.fileNum)
	file, err := os.OpenFile(filepath.Join(s.dataDir, filename), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *Provider) OpenFileForReading(meta *FileMetadata) (*os.File, error) {
	filename := s.makeFileName(meta.fileNum)
	file, err := os.OpenFile(filepath.Join(s.dataDir, filename), os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return file, nil
}
