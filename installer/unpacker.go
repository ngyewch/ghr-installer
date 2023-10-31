package installer

import (
	"context"
	"fmt"
	"github.com/mholt/archiver/v4"
	"io"
	"os"
	"path/filepath"
)

func unpack(archivePath string, installDirectory string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	var input io.Reader = f
	format, input, err := archiver.Identify(archivePath, input)
	if err != nil {
		return err
	}

	decompressor, ok := format.(archiver.Decompressor)
	if ok {
		r, err := decompressor.OpenReader(input)
		if err != nil {
			return err
		}
		defer func(r io.ReadCloser) {
			_ = r.Close()
		}(r)

		format2, input2, err := archiver.Identify("", r)
		if err != nil {
			return err
		}

		format = format2
		input = input2
	}

	extractor, ok := format.(archiver.Extractor)
	if !ok {
		return fmt.Errorf("could not extract archive")
	}
	err = extractor.Extract(context.Background(), input, nil,
		func(ctx context.Context, f archiver.File) error {
			if f.LinkTarget != "" {
				outputPath := filepath.Join(installDirectory, f.NameInArchive)
				outputDirectory := filepath.Dir(outputPath)
				err = os.MkdirAll(outputDirectory, 0755)
				if err != nil {
					return err
				}

				err = os.Symlink(f.LinkTarget, outputPath)
				if err != nil {
					return err
				}

				return nil
			}

			if f.IsDir() {
				outputPath := filepath.Join(installDirectory, f.NameInArchive)
				err = os.MkdirAll(outputPath, f.Mode())
				if err != nil {
					return err
				}

				return nil
			}

			r, err := f.Open()
			if err != nil {
				return err
			}
			defer func(r io.ReadCloser) {
				_ = r.Close()
			}(r)

			outputPath := filepath.Join(installDirectory, f.NameInArchive)
			outputDirectory := filepath.Dir(outputPath)
			err = os.MkdirAll(outputDirectory, 0755)
			if err != nil {
				return err
			}

			w, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE, f.Mode())
			if err != nil {
				return err
			}
			defer func(w *os.File) {
				_ = w.Close()
			}(w)

			_, err = io.Copy(w, r)
			if err != nil {
				return err
			}

			return nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}
