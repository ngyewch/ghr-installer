package installer

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

type FileChecksum struct {
	Checksum []byte
	Filename string
}

type FileChecksums struct {
	Entries []*FileChecksum
}

func ReadFileChecksumsFromFile(path string) (*FileChecksums, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	return ReadFileChecksums(f)
}

func ReadFileChecksums(r io.Reader) (*FileChecksums, error) {
	var fileChecksums FileChecksums

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "  ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format")
		}
		checksum, err := hex.DecodeString(parts[0])
		if err != nil {
			return nil, err
		}
		fileChecksums.Entries = append(fileChecksums.Entries, &FileChecksum{
			Checksum: checksum,
			Filename: parts[1],
		})
	}

	return &fileChecksums, nil
}

func (checksums *FileChecksums) GetEntry(filename string) *FileChecksum {
	for _, entry := range checksums.Entries {
		if entry.Filename == filename {
			return entry
		}
	}
	return nil
}

func CalcFileChecksum(path string, alg string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	return CalcChecksum(f, alg)
}

func CalcChecksum(r io.Reader, alg string) ([]byte, error) {
	var hasher hash.Hash
	switch alg {
	case "md5":
		hasher = md5.New()
	case "sha1":
		hasher = sha1.New()
	case "sha224":
		hasher = sha256.New224()
	case "sha256":
		hasher = sha256.New()
	case "sha384":
		hasher = sha512.New384()
	case "sha512":
		hasher = sha512.New()
	default:
		return nil, fmt.Errorf("unsupported algorithm")
	}

	_, err := io.Copy(hasher, r)
	if err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}
