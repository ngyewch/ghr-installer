package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v56/github"
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

type Installer struct {
	baseDirectory string
	githubClient  *github.Client
}

func NewInstaller(baseDirectory string, githubClient *github.Client) *Installer {
	return &Installer{
		baseDirectory: baseDirectory,
		githubClient:  githubClient,
	}
}

func (installer *Installer) Install(packageSpec string) (bool, error) {
	changed := false

	pkgSpec, err := ParsePackageSpec(packageSpec)
	if err != nil {
		return false, err
	}

	repositoryRelease, metadataChanged, err := installer.getRepositoryRelease(pkgSpec)
	if err != nil {
		return false, err
	}
	changed = changed || metadataChanged

	packageAsset, packageBaseName := func() (*github.ReleaseAsset, string) {
		for _, asset := range repositoryRelease.Assets {
			match, baseName := matchPackageFilename(pkgSpec, asset.GetName())
			if match {
				return asset, baseName
			}
		}
		return nil, ""
	}()
	if packageAsset == nil {
		return changed, fmt.Errorf("could not locate package for this os/arch")
	}

	checksumsAsset, checksumAlgorithm, checksumForContent := func() (*github.ReleaseAsset, string, bool) {
		for _, asset := range repositoryRelease.Assets {
			match, alg, isContentChecksum := matchChecksumFilename(pkgSpec, packageBaseName, asset.GetName())
			if match {
				return asset, alg, isContentChecksum
			}
		}
		return nil, "", false
	}()

	if packageAsset != nil {
		packagePath, packageChanged, err := installer.downloadAsset(pkgSpec, "package", packageAsset)
		if err != nil {
			return changed, err
		}
		changed = changed || packageChanged

		checksums, err := func() (*Checksums, error) {
			if checksumsAsset == nil {
				return nil, nil
			}
			checksumsPath, checksumsChanged, err := installer.downloadAsset(pkgSpec, "checksums", checksumsAsset)
			if err != nil {
				return nil, err
			}
			changed = changed || checksumsChanged
			return ReadFileChecksumsFromFile(checksumAlgorithm, checksumsPath)
		}()
		if err != nil {
			return changed, err
		}

		if (checksums != nil) && !checksumForContent {
			count, err := checksums.Check(installer.downloadDirectory(pkgSpec))
			if err != nil {
				return changed, err
			}
			if count > 0 {
				fmt.Printf("[%s] %d checksums matched\n", pkgSpec, count)
			} else {
				fmt.Printf("[%s] (WARNING) %d checksums matched\n", pkgSpec, count)
			}
		}

		fmt.Printf("[%s] extracting...\n", packageSpec)
		installDirectory := installer.installDirectory(pkgSpec)
		fileCount, err := unpack(packagePath, installDirectory)
		if err != nil {
			return changed, err
		}
		changed = changed || (fileCount > 0)

		if (checksums != nil) && checksumForContent {
			count, err := checksums.Check(installer.installDirectory(pkgSpec))
			if err != nil {
				return changed, err
			}
			if count > 0 {
				fmt.Printf("[%s] %d checksums matched\n", pkgSpec, count)
			} else {
				fmt.Printf("[%s] (WARNING) %d checksums matched\n", pkgSpec, count)
			}
		}
	}

	return changed, nil
}

func (installer *Installer) metadataDirectory(pkgSpec *PackageSpec) string {
	return filepath.Join(installer.baseDirectory, "metadata", "github.com", pkgSpec.Owner, pkgSpec.Project, pkgSpec.Version)
}

func (installer *Installer) downloadDirectory(pkgSpec *PackageSpec) string {
	return filepath.Join(installer.baseDirectory, "downloads", "github.com", pkgSpec.Owner, pkgSpec.Project, pkgSpec.Version)
}

func (installer *Installer) installDirectory(pkgSpec *PackageSpec) string {
	return filepath.Join(installer.baseDirectory, "installs", "github.com", pkgSpec.Owner, pkgSpec.Project, pkgSpec.Version)
}

func (installer *Installer) getRepositoryRelease(pkgSpec *PackageSpec) (*github.RepositoryRelease, bool, error) {
	metadataDirectory := installer.metadataDirectory(pkgSpec)
	err := os.MkdirAll(metadataDirectory, 0755)
	if err != nil {
		return nil, false, err
	}
	repositoryReleaseFile := filepath.Join(metadataDirectory, "repositoryRelease.json")
	jsonBytes, err := os.ReadFile(repositoryReleaseFile)
	if os.IsNotExist(err) {
		repositoryRelease, _, err := installer.githubClient.Repositories.GetReleaseByTag(context.Background(), pkgSpec.Owner, pkgSpec.Project, fmt.Sprintf("v%s", pkgSpec.Version))
		if err != nil {
			return nil, false, err
		}
		jsonBytes, err := json.MarshalIndent(repositoryRelease, "", "  ")
		if err != nil {
			return nil, false, err
		}
		err = os.WriteFile(repositoryReleaseFile, jsonBytes, 0755)
		if err != nil {
			return nil, false, err
		}
		return repositoryRelease, true, nil
	} else if err != nil {
		return nil, false, err
	}
	var repositoryRelease github.RepositoryRelease
	err = json.Unmarshal(jsonBytes, &repositoryRelease)
	if err != nil {
		return nil, false, err
	}
	return &repositoryRelease, false, nil
}

func (installer *Installer) downloadAsset(pkgSpec *PackageSpec, assetType string, asset *github.ReleaseAsset) (string, bool, error) {
	downloadDirectory := installer.downloadDirectory(pkgSpec)

	downloadFile := filepath.Join(downloadDirectory, asset.GetName())
	_, err := os.Stat(downloadFile)
	if err == nil {
		fmt.Printf("[%s] %s already downloaded...\n", pkgSpec, assetType)
		return downloadFile, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, err
	}

	err = os.MkdirAll(downloadDirectory, 0755)
	if err != nil {
		return "", false, err
	}

	fmt.Printf("[%s] downloading %s (%s)...\n", pkgSpec, assetType, asset.GetBrowserDownloadURL())

	httpResponse, err := http.Get(asset.GetBrowserDownloadURL())
	if err != nil {
		return "", false, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(httpResponse.Body)

	f, err := os.Create(downloadFile)
	if err != nil {
		return "", false, err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	_, err = io.Copy(f, httpResponse.Body)
	if err != nil {
		return "", false, err
	}

	return downloadFile, true, nil
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

func matchChecksumFilename(pkgSpec *PackageSpec, packageBaseName string, name string) (bool, string, bool) {
	// TODO always assume these patterns?
	if name == "checksums.txt" {
		return true, "", false
	}
	if strings.EqualFold(name, "SHASUMS256.txt") {
		return true, "sha256", false
	}
	if strings.EqualFold(name, "SHASUMS512.txt") {
		return true, "sha512", false
	}
	for _, hashAlgorithm := range hashAlgorithms {
		if strings.EqualFold(name, fmt.Sprintf("%ssums.txt", hashAlgorithm)) {
			return true, hashAlgorithm, false
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
		return matched, alg, false
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
		return matched, alg, true
	}

	return false, "", false
}
