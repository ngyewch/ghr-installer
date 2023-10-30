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
	"slices"
	"strings"
)

var (
	allowedExts = []string{ // TODO other extensions?
		".tar.gz",
		".tar.bz2",
		".tar.xz",
		".tar",
		".zip",
		".rar",
		".7z",
	}
	delims         = []string{".", "-", "_"}
	hashAlgorithms = []string{"md5", "sha1", "sha224", "sha256", "sha384", "sha512"}
)

func Install(baseDirectory string, packageSpec string) error {
	pkgSpec, err := ParsePackageSpec(packageSpec)
	if err != nil {
		return err
	}

	githubClient := github.NewClient(nil)

	var packageAsset *github.ReleaseAsset
	var packageBaseName string
	release, _, err := githubClient.Repositories.GetReleaseByTag(context.Background(), pkgSpec.Owner, pkgSpec.Project, fmt.Sprintf("v%s", pkgSpec.Version))
	if err != nil {
		return err
	}
	for _, asset := range release.Assets {
		match, baseName := matchPackageFilename(pkgSpec, asset.GetName())
		if match {
			packageBaseName = baseName
			packageAsset = asset
			break
		}
	}
	if packageAsset == nil {
		return fmt.Errorf("could not locate package for this os/arch")
	}

	var checksumsAsset *github.ReleaseAsset
	var checksumAlgorithm string
	for _, asset := range release.Assets {
		match, alg := matchChecksumFilename(pkgSpec, packageBaseName, asset.GetName())
		if match {
			checksumsAsset = asset
			checksumAlgorithm = alg
			break
		}
	}

	downloadDirectory := filepath.Join(baseDirectory, "downloads", "github.com", pkgSpec.Owner, pkgSpec.Project, pkgSpec.Version)

	if packageAsset != nil {
		packagePath, err := downloadAsset(pkgSpec, "package", packageAsset, downloadDirectory)
		if err != nil {
			return err
		}

		if checksumsAsset != nil {
			fileChecksumsPath, err := downloadAsset(pkgSpec, "checksums", checksumsAsset, downloadDirectory)
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
				alg := checksumAlgorithm
				if alg == "" {
					switch len(fileChecksumEntry.Checksum) {
					case 16:
						alg = "md5"
					case 20:
						alg = "sha1"
					case 28:
						alg = "sha224"
					case 32:
						alg = "sha256"
					case 48:
						alg = "sha384"
					case 64:
						alg = "sha512"
					default:
						return fmt.Errorf("could not auto-detect checksum algorithm")
					}
				}
				fmt.Printf("[%s] comparing checksums...\n", packageSpec)
				checksum, err := CalcFileChecksum(packagePath, alg)
				if err != nil {
					return err
				}
				if !bytes.Equal(checksum, fileChecksumEntry.Checksum) {
					return fmt.Errorf("checksum mismatch")
				}
			} else {
				return fmt.Errorf("could not find checksum entry")
			}
		}

		fmt.Printf("[%s] installing...\n", packageSpec)
		installDirectory := filepath.Join(baseDirectory, "installs", "github.com", pkgSpec.Owner, pkgSpec.Project, pkgSpec.Version)
		err = unpack(packagePath, installDirectory)
		if err != nil {
			return err
		}
	}

	return nil
}

func matchPackageFilename(pkgSpec *PackageSpec, name string) (bool, string) {
	expecter := NewExpecter(name)
	if !expecter.ExpectString(pkgSpec.Project) { // TODO always assume projectName?
		return false, ""
	}
	if !expecter.ExpectStrings(delims) {
		return false, ""
	}
	if !expecter.ExpectStrings([]string{pkgSpec.Version, "v" + pkgSpec.Version}) {
		return false, ""
	}
	if !expecter.ExpectStrings(delims) {
		return false, ""
	}
	osClassifiers := []string{
		runtime.GOOS,
	}
	if !expecter.ExpectStrings(osClassifiers) {
		return false, ""
	}
	if !expecter.ExpectStrings(delims) {
		return false, ""
	}
	archClassifiers := []string{
		runtime.GOARCH,
	}
	if runtime.GOARCH == "amd64" {
		archClassifiers = append(archClassifiers, "64bit", "x64")
	}
	if !expecter.ExpectStrings(archClassifiers) {
		return false, ""
	}
	if !expecter.ExpectString(".") {
		return false, ""
	}

	matched := expecter.Matched()
	matched = matched[0 : len(matched)-1]

	ext := name[len(matched):]
	if !slices.Contains(allowedExts, ext) {
		return false, ""
	}

	return true, matched
}

func matchChecksumFilename(pkgSpec *PackageSpec, packageBaseName string, name string) (bool, string) {
	// TODO always assume these patterns?
	if name == "checksums.txt" {
		return true, ""
	}
	if strings.EqualFold(name, "SHASUMS256.txt") {
		return true, "sha256"
	}
	if strings.EqualFold(name, "SHASUMS512.txt") {
		return true, "sha512"
	}
	for _, hashAlgorithm := range hashAlgorithms {
		if strings.EqualFold(name, fmt.Sprintf("%ssums.txt", hashAlgorithm)) {
			return true, hashAlgorithm
		}
	}

	matched, alg := func() (bool, string) {
		expecter := NewExpecter(name)
		if !expecter.ExpectString(pkgSpec.Project) {
			return false, ""
		}
		if !expecter.ExpectStrings(delims) {
			return false, ""
		}
		if !expecter.ExpectStrings([]string{pkgSpec.Version, "v" + pkgSpec.Version}) {
			return false, ""
		}
		if !expecter.ExpectStrings(delims) {
			return false, ""
		}
		if !expecter.ExpectString("checksums.txt") {
			return false, ""
		}
		return true, ""
	}()
	if matched {
		return matched, alg
	}

	matched, alg = func() (bool, string) {
		expecter := NewExpecter(name)
		if !expecter.ExpectString(packageBaseName) {
			return false, ""
		}
		if !expecter.ExpectStrings(delims) {
			return false, ""
		}
		for _, hashAlgorithm := range hashAlgorithms {
			if expecter.PeekString(hashAlgorithm) {
				if !expecter.ExpectString(hashAlgorithm) {
					return false, ""
				}
				if !expecter.ExpectString("sum.txt") {
					return false, ""
				}
				if !expecter.IsEmpty() {
					return false, ""
				}
				return true, hashAlgorithm
			}
		}
		return false, ""
	}()
	if matched {
		return matched, alg
	}

	return false, ""
}

func downloadAsset(packageSpec *PackageSpec, assetType string, asset *github.ReleaseAsset, downloadDirectory string) (string, error) {
	err := os.MkdirAll(downloadDirectory, 0755)
	if err != nil {
		return "", err
	}

	downloadFile := filepath.Join(downloadDirectory, asset.GetName())
	_, err = os.Stat(downloadFile)
	if err == nil {
		fmt.Printf("[%s] %s already downloaded...\n", packageSpec, assetType)
		return downloadFile, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	fmt.Printf("[%s] downloading %s (%s)...\n", packageSpec, assetType, asset.GetBrowserDownloadURL())

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
			if f.LinkTarget != "" {
				outputPath := filepath.Join(installDirectory, f.Name())
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
				outputPath := filepath.Join(installDirectory, f.Name())
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

			outputPath := filepath.Join(installDirectory, f.Name())
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
