package loader

import "context"

type FileType string

const (
	FileTypePDF  FileType = "pdf"
	FileTypeDOCX FileType = "docx"
	FileTypeMD   FileType = "md"
	FileTypeTXT  FileType = "txt"
)

type FileDescriptor struct {
	Path     string
	FileType FileType
}

type Surveyor interface {
	Survey(ctx context.Context, dir string) ([]*FileDescriptor, error)
}
