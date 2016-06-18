package p2p

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

type MetaInfoFileSystem interface {
	Open(name string) (MetaInfoFile, error)
	Stat(name string) (os.FileInfo, error)
}

type MetaInfoFile interface {
	io.Closer
	io.Reader
	io.ReaderAt
	Readdirnames(n int) (names []string, err error)
	Stat() (os.FileInfo, error)
}

// Adapt a MetaInfoFileSystem into a torrent file store FileSystem
type FileStoreFileSystemAdapter struct {
}

type FileStoreFileAdapter struct {
	f MetaInfoFile
}

func (f *FileStoreFileSystemAdapter) Open(name []string, length int64) (file File, err error) {
	var ff MetaInfoFile
	ff, err = os.Open(path.Join(name...))
	if err != nil {
		return
	}
	stat, err := ff.Stat()
	if err != nil {
		return
	}
	actualSize := stat.Size()
	if actualSize != length {
		err = fmt.Errorf("Unexpected file size %v. Expected %v", actualSize, length)
		return
	}
	file = &FileStoreFileAdapter{ff}
	return
}

func (f *FileStoreFileSystemAdapter) Close() error {
	return nil
}

func (f *FileStoreFileAdapter) ReadAt(p []byte, off int64) (n int, err error) {
	return f.f.ReadAt(p, off)
}

func (f *FileStoreFileAdapter) WriteAt(p []byte, off int64) (n int, err error) {
	// Writes must match existing data exactly.
	q := make([]byte, len(p))
	_, err = f.ReadAt(q, off)
	if err != nil {
		return
	}
	if bytes.Compare(p, q) != 0 {
		err = fmt.Errorf("New data does not match original data.")
	}
	return
}

func (f *FileStoreFileAdapter) Close() (err error) {
	return f.f.Close()
}

func (m *MetaInfo) addFiles(fileInfo os.FileInfo, file string) (err error) {
	if len(m.Files) == 0 {
		m.Files = make([]FileDict, 1)
	}

	fileDict := FileDict{Length: fileInfo.Size()}
	cleanFile := filepath.Clean(file)
	fileDict.Path, fileDict.Name = path.Split(cleanFile)
	fileDict.Sum, err = sha1Sum(file)
	if err != nil {
		return err
	}
	m.Files = append(m.Files, fileDict)
	return
}

func CreateFileMeta(roots []string, pieceLen int64) (mi *MetaInfo, err error) {
	mi = &MetaInfo{}
	for _, f := range roots {
		var fileInfo os.FileInfo
		fileInfo, err = os.Stat(f)
		if err != nil {
			return
		}

		if fileInfo.IsDir() {
			return nil, fmt.Errorf("Not support dir")
		}

		err = mi.addFiles(fileInfo, f)
		if err != nil {
			return nil, err
		}
		mi.Length += fileInfo.Size()
	}

	if pieceLen == 0 {
		pieceLen = choosePieceLength(mi.Length)
	}
	mi.PieceLen = pieceLen

	fileStoreFS := &FileStoreFileSystemAdapter{}
	var fileStore FileStore
	fileStore, err = NewFileStore(mi, fileStoreFS)
	if err != nil {
		return nil, err
	}

	var sums []byte
	sums, err = computeSums(fileStore, mi.Length, mi.PieceLen)
	if err != nil {
		return nil, err
	}
	mi.Pieces = string(sums)
	return mi, nil
}

func sha1Sum(file string) (sum string, err error) {
	var f MetaInfoFile
	f, err = os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	hash := sha1.New()
	_, err = io.Copy(hash, f)
	if err != nil {
		return
	}
	sum = string(hash.Sum(nil))
	return
}

const (
	MinimumPieceLength   = 16 * 1024
	TargetPieceCountLog2 = 10
	TargetPieceCountMin  = 1 << TargetPieceCountLog2

	// Target piece count should be < TargetPieceCountMax
	TargetPieceCountMax = TargetPieceCountMin << 1
)

// Choose a good piecelength.
func choosePieceLength(totalLength int64) (pieceLength int64) {
	// Must be a power of 2.
	// Must be a multiple of 16KB
	// Prefer to provide around 1024..2048 pieces.
	pieceLength = MinimumPieceLength
	pieces := totalLength / pieceLength
	for pieces >= TargetPieceCountMax {
		pieceLength <<= 1
		pieces >>= 1
	}
	return
}
