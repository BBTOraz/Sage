package loader

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type LocalSurveyor struct {
	exts map[string]FileType
}

func NewSurveyor(exts map[string]FileType) *LocalSurveyor {
	return &LocalSurveyor{exts: exts}
}

func (s *LocalSurveyor) Survey(ctx context.Context, dir string) (desc []*FileDescriptor, err error) {
	desc = make([]*FileDescriptor, 0, 100)
	dir = filepath.Clean(dir)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			return err
		}

		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		fileType, ok := s.exts[ext]
		if !ok {
			return nil
		}

		desc = append(desc, &FileDescriptor{
			Path:     path,
			FileType: fileType,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return desc, nil
}
