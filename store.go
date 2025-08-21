package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
//                          PATH TRANSFORM (CAS)                              //
////////////////////////////////////////////////////////////////////////////////

const defaultRootFolderName = "ggnetwork"

// CASPathTransformFunc: hàm chuyển đổi key → đường dẫn theo cơ chế CAS (Content Addressable Storage).
// Ý tưởng: thay vì lưu file trực tiếp theo tên key, ta hash nó (SHA-1).
// → hash được cắt thành nhiều đoạn để tạo cây thư mục, tránh việc có hàng ngàn file trong 1 folder.
func CASPathTransformFunc(key string) PathKey {
	// Tạo SHA-1 hash từ key
	hash := sha1.Sum([]byte(key))
	hashStr := hex.EncodeToString(hash[:]) // chuyển sang chuỗi hex

	blocksize := 5 // mỗi thư mục con chứa 5 ký tự hash
	sliceLen := len(hashStr) / blocksize
	paths := make([]string, sliceLen)

	// Cắt chuỗi hash thành các phần nhỏ để làm cây thư mục
	for i := 0; i < sliceLen; i++ {
		from, to := i*blocksize, (i*blocksize)+blocksize
		paths[i] = hashStr[from:to]
	}

	// Trả về PathKey với:
	// - PathName: thư mục lồng nhau
	// - Filename: tên file chính là toàn bộ chuỗi hash
	return PathKey{
		PathName: strings.Join(paths, "/"),
		Filename: hashStr,
	}
}

// PathTransformFunc là kiểu hàm chung: nhận vào string (key) → trả về PathKey.
type PathTransformFunc func(string) PathKey

// PathKey chứa thông tin đường dẫn của file.
type PathKey struct {
	PathName string // đường dẫn (đã cắt từ hash thành nhiều folder con)
	Filename string // tên file (hash đầy đủ)
}

// FirstPathName: lấy thư mục con đầu tiên trong chuỗi path.
// Dùng để xóa nguyên "cây con" trong Delete().
func (p PathKey) FirstPathName() string {
	paths := strings.Split(p.PathName, "/")
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// FullPath: ghép PathName + Filename thành đường dẫn đầy đủ của file (chưa có Root, ID).
func (p PathKey) FullPath() string {
	return fmt.Sprintf("%s/%s", p.PathName, p.Filename)
}

////////////////////////////////////////////////////////////////////////////////
//                                STORE                                       //
////////////////////////////////////////////////////////////////////////////////

// StoreOpts: cấu hình khi tạo Store
type StoreOpts struct {
	Root              string            // Thư mục gốc chứa toàn bộ dữ liệu
	PathTransformFunc PathTransformFunc // Hàm chuyển đổi key → PathKey (nếu nil → mặc định)
}

// DefaultPathTransformFunc: cách map key → path đơn giản (key = filename, không hash)
var DefaultPathTransformFunc = func(key string) PathKey {
	return PathKey{
		PathName: key,
		Filename: key,
	}
}

// Store: đại diện cho "kho lưu trữ" trên ổ đĩa.
// Nó dùng Root để lưu file, và PathTransformFunc để map key → đường dẫn file.
type Store struct {
	StoreOpts
}

// NewStore: khởi tạo Store mới với cấu hình.
func NewStore(opts StoreOpts) *Store {
	// Nếu không có PathTransformFunc thì dùng mặc định
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	// Nếu không có Root thì đặt mặc định là "ggnetwork"
	if len(opts.Root) == 0 {
		opts.Root = defaultRootFolderName
	}

	return &Store{
		StoreOpts: opts,
	}
}

////////////////////////////////////////////////////////////////////////////////
//                           STORE METHODS                                    //
////////////////////////////////////////////////////////////////////////////////

// Has: kiểm tra xem file có tồn tại trên đĩa không.
func (s *Store) Has(id string, key string) bool {
	// Tạo pathKey từ key
	pathKey := s.PathTransformFunc(key)
	// Đường dẫn đầy đủ: Root/ID/Path/Filename
	fullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())

	_, err := os.Stat(fullPathWithRoot)
	// Nếu file không tồn tại → trả về false
	return !errors.Is(err, os.ErrNotExist)
}

// Clear: xóa toàn bộ thư mục Root (dọn sạch store)
func (s *Store) Clear() error {
	return os.RemoveAll(s.Root)
}

// Delete: xóa file (hoặc cả cây thư mục con liên quan đến key)
// Ở đây nó xóa từ "FirstPathName", nghĩa là xóa luôn nhóm file cùng nhánh.
func (s *Store) Delete(id string, key string) error {
	pathKey := s.PathTransformFunc(key)

	defer func() {
		log.Printf("deleted [%s] from disk", pathKey.Filename)
	}()

	// Xóa nguyên folder con đầu tiên chứa file này
	firstPathNameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FirstPathName())
	return os.RemoveAll(firstPathNameWithRoot)
}

// Write: ghi dữ liệu từ io.Reader vào file (không mã hóa).
func (s *Store) Write(id string, key string, r io.Reader) (int64, error) {
	return s.writeStream(id, key, r)
}

// WriteDecrypt: ghi dữ liệu từ io.Reader vào file, với dữ liệu đã mã hóa (AES).
// Nó sẽ giải mã (decrypt) trước khi ghi ra đĩa.
func (s *Store) WriteDecrypt(encKey []byte, id string, key string, r io.Reader) (int64, error) {
	f, err := s.openFileForWriting(id, key)
	if err != nil {
		return 0, err
	}
	// copyDecrypt vừa giải mã vừa ghi ra file
	n, err := copyDecrypt(encKey, r, f)
	return int64(n), err
}

// openFileForWriting: mở file để ghi (tạo thư mục cha nếu chưa có).
func (s *Store) openFileForWriting(id string, key string) (*os.File, error) {
	pathKey := s.PathTransformFunc(key)
	// Tạo cây thư mục (nếu chưa có)
	pathNameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.PathName)
	if err := os.MkdirAll(pathNameWithRoot, os.ModePerm); err != nil {
		return nil, err
	}

	fullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())
	// Trả về file handle (tạo mới file)
	return os.Create(fullPathWithRoot)
}

// writeStream: hàm phụ cho Write (copy dữ liệu từ Reader → file).
func (s *Store) writeStream(id string, key string, r io.Reader) (int64, error) {
	f, err := s.openFileForWriting(id, key)
	if err != nil {
		return 0, err
	}
	return io.Copy(f, r)
}

// Read: đọc dữ liệu từ file ra (trả về io.Reader để stream).
func (s *Store) Read(id string, key string) (int64, io.Reader, error) {
	return s.readStream(id, key)
}

// readStream: mở file và trả về io.ReadCloser cùng với kích thước file.
func (s *Store) readStream(id string, key string) (int64, io.ReadCloser, error) {
	pathKey := s.PathTransformFunc(key)
	fullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())

	// Mở file
	file, err := os.Open(fullPathWithRoot)
	if err != nil {
		return 0, nil, err
	}

	// Lấy thông tin file (size, etc.)
	fi, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}

	// Trả về size và file (dùng làm Reader)
	return fi.Size(), file, nil
}
