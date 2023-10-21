package installer

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/go-github/v56/github"
	"github.com/mholt/archiver/v4"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func Install(packageSpec string) error {
	pkgSpec, err := ParsePackageSpec(packageSpec)
	if err != nil {
		return err
	}

	osClassifier := runtime.GOOS     // TODO always assume runtime.GOOS?
	archClassifier := runtime.GOARCH // TODO always assume runtime.GOARCH?
	// TODO always assume this format?
	// TODO always assume .tar.gz extension?
	baseDownloadFilename := fmt.Sprintf("%s_%s_%s_%s",
		pkgSpec.Project, // TODO always assume filename == projectName?
		pkgSpec.Version, // TODO always assume no prefix?
		osClassifier,
		archClassifier,
	)
	downloadFilename := baseDownloadFilename + ".tar.gz"
	downloadUrl := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/%s",
		pkgSpec.Owner,
		pkgSpec.Project,
		pkgSpec.Version, // TODO always assume 'v' prefix?
		downloadFilename,
	)

	fmt.Println(downloadUrl)

	githubClient := github.NewClient(nil)

	var packageAsset *github.ReleaseAsset
	release, _, err := githubClient.Repositories.GetReleaseByTag(context.Background(), pkgSpec.Owner, pkgSpec.Project, fmt.Sprintf("v%s", pkgSpec.Version))
	if err != nil {
		return err
	}
	prefix := baseDownloadFilename + "."
	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.GetName(), prefix) {
			ext := asset.GetName()[len(baseDownloadFilename):]
			if (ext == ".tar.gz") || (ext == ".zip") { // TODO other extensions?
				fmt.Printf("[download] %s -> %s\n", asset.GetName(), asset.GetBrowserDownloadURL())
				packageAsset = asset
				break
			}
		}
	}

	var checksumsAsset *github.ReleaseAsset
	checksumsFilename := fmt.Sprintf("%s_%s_checksums.txt", pkgSpec.Project, pkgSpec.Version)
	for _, asset := range release.Assets {
		if asset.GetName() == checksumsFilename {
			fmt.Printf("[checksums] %s -> %s\n", asset.GetName(), asset.GetBrowserDownloadURL())
			checksumsAsset = asset
			break
		}
	}

	baseDirectory := filepath.Join("dist", "store")

	downloadDirectory := filepath.Join(baseDirectory, "downloads", "github.com", pkgSpec.Owner, pkgSpec.Project, pkgSpec.Version)

	if packageAsset != nil {
		packagePath, err := downloadAsset(packageAsset, downloadDirectory)
		if err != nil {
			return err
		}

		if checksumsAsset != nil {
			fileChecksumsPath, err := downloadAsset(checksumsAsset, downloadDirectory)
			if err != nil {
				return err
			}
			fileChecksums, err := ReadFileChecksumsFromFile(fileChecksumsPath)
			if err != nil {
				return err
			}
			fileChecksumEntry := fileChecksums.GetEntry(packageAsset.GetName())
			if err != nil {
				return err
			}
			if fileChecksumEntry != nil {
				checksum, err := CalcFileChecksum(packagePath, "sha512")
				if err != nil {
					return err
				}
				if !bytes.Equal(checksum, fileChecksumEntry.Checksum) {
					return fmt.Errorf("checksum mismatch")
				}
			}
		}

		installDirectory := filepath.Join(baseDirectory, "installs", "github.com", pkgSpec.Owner, pkgSpec.Project, pkgSpec.Version)
		err = unpack(packagePath, installDirectory)
		if err != nil {
			return err
		}
	}

	return nil
}

func downloadAsset(asset *github.ReleaseAsset, downloadDirectory string) (string, error) {
	err := os.MkdirAll(downloadDirectory, 0755)
	if err != nil {
		return "", err
	}

	downloadFile := filepath.Join(downloadDirectory, asset.GetName())
	_, err = os.Stat(downloadFile)
	if err == nil {
		fmt.Printf("skipping %s...\n", asset.GetName())
	} else if !os.IsNotExist(err) {
		return "", err
	}

	httpResponse, err := http.Get(asset.GetBrowserDownloadURL())
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(httpResponse.Body)

	f, err := os.Create(downloadFile)
	if err != nil {
		return "", err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	_, err = io.Copy(f, httpResponse.Body)
	if err != nil {
		return "", err
	}

	return downloadFile, nil
}

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
			if f.IsDir() {
				return nil
			}

			r, err := f.Open()
			if err != nil {
				return err
			}
			defer func(r io.ReadCloser) {
				_ = r.Close()
			}(r)

			outputPath := filepath.Join(installDirectory, f.Name())
			outputDirectory := filepath.Dir(outputPath)
			err = os.MkdirAll(outputDirectory, 0755)
			if err != nil {
				return err
			}

			w, err := os.Create(outputPath)
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
