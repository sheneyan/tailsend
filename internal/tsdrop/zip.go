package tsdrop

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func writeZip(w io.Writer, root string) error {
	zw := zip.NewWriter(w)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		if info.IsDir() {
			_, err := zw.Create(name + "/")
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		fh, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		fh.Name = name
		fh.Method = zip.Deflate
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(fw, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	closeErr := zw.Close()
	if err != nil {
		return err
	}
	return closeErr
}
