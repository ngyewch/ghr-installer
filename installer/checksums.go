package installer

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ChecksumEntry struct {
	Checksum []byte
	Filename string
}

type Checksums struct {
	Algorithm string
	Entries   []*ChecksumEntry
}

func ReadFileChecksumsFromFile(algorithm string, path string) (*Checksums, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	return ReadFileChecksums(algorithm, f)
}

func ReadFileChecksums(algorithm string, r io.Reader) (*Checksums, error) {
	checksums := &Checksums{
		Algorithm: algorithm,
	}

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
		checksums.Entries = append(checksums.Entries, &ChecksumEntry{
			Checksum: checksum,
			Filename: parts[1],
		})
	}

	return checksums, nil
}

func (checksums *Checksums) Check(directory string) (int, error) {
	count := 0
	for _, entry := range checksums.Entries {
		path := filepath.Join(directory, entry.Filename)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			// do nothing
		} else if err != nil {
			return count, err
		} else {
			alg := checksums.Algorithm
			if alg == "" {
				alg = DetectChecksumAlgorithm(entry.Checksum)
			}
			if alg == "" {
				return count, fmt.Errorf("could not detect checksum algorithm")
			}
			checksum, err := CalcFileChecksum(path, alg)
			if err != nil {
				return count, err
			}
			if !bytes.Equal(checksum, entry.Checksum) {
				return count, fmt.Errorf("checksum mismatch")
			}
			count++
		}
	}
	return count, nil
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
	hasher := NewHasher(alg)
	if hasher == nil {
		return nil, fmt.Errorf("unsupported hash algorithm")
	}

	_, err := io.Copy(hasher, r)
	if err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}

func NewHasher(alg string) hash.Hash {
	switch alg {
	case "md5":
		return md5.New()
	case "sha1":
		return sha1.New()
	case "sha224":
		return sha256.New224()
	case "sha256":
		return sha256.New()
	case "sha384":
		return sha512.New384()
	case "sha512":
		return sha512.New()
	default:
		return nil
	}
}

func DetectChecksumAlgorithm(checksum []byte) string {
	switch len(checksum) {
	case 16:
		return "md5"
	case 20:
		return "sha1"
	case 28:
		return "sha224"
	case 32:
		return "sha256"
	case 48:
		return "sha384"
	case 64:
		return "sha512"
	default:
		return ""
	}
}
