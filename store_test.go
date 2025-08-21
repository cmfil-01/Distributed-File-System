package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
)

////////////////////////////////////////////////////////////////////////////////
//                          TEST PATH TRANSFORM FUNC                          //
////////////////////////////////////////////////////////////////////////////////

// TestPathTransformFunc kiểm tra hàm CASPathTransformFunc (Content-Addressable Storage).
// Ý tưởng: từ 1 key bất kỳ → hash (SHA-1) →
//   - Filename: chuỗi hash đầy đủ (ví dụ: 6804429f74....353ff)
//   - PathName: hash được chia nhỏ thành nhiều thư mục con để tránh dồn nhiều file vào 1 folder.
func TestPathTransformFunc(t *testing.T) {
	key := "momsbestpicture"

	// Thực hiện transform key → pathKey
	pathKey := CASPathTransformFunc(key)

	// Expected: kết quả hash cố định (SHA-1 của "momsbestpicture")
	expectedFilename := "6804429f74181a63c50c3d81d733a12f14a353ff"
	// Expected path: cắt hash thành từng block 5 ký tự → tạo cây thư mục
	expectedPathName := "68044/29f74/181a6/3c50c/3d81d/733a1/2f14a/353ff"

	// So sánh kết quả thực tế với expected
	if pathKey.PathName != expectedPathName {
		t.Errorf("have %s want %s", pathKey.PathName, expectedPathName)
	}

	if pathKey.Filename != expectedFilename {
		t.Errorf("have %s want %s", pathKey.Filename, expectedFilename)
	}
}

////////////////////////////////////////////////////////////////////////////////
//                           TEST STORE (READ/WRITE)                          //
////////////////////////////////////////////////////////////////////////////////

// TestStore kiểm tra toàn bộ vòng đời của 1 file trong Store:
// - Ghi file
// - Kiểm tra tồn tại
// - Đọc lại
// - Xóa
// - Kiểm tra không còn tồn tại
func TestStore(t *testing.T) {
	// Tạo store mới (dùng CAS transform)
	s := newStore()

	// Tạo ID giả lập cho "node" (node ID dùng để phân vùng lưu trữ)
	id := generateID()

	// Dọn sạch store sau khi test xong
	defer teardown(t, s)

	// Test với 50 keys
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("foo_%d", i)
		data := []byte("some jpg bytes") // dữ liệu giả lập (thay vì ảnh thật)

		// --- WRITE ---
		if _, err := s.writeStream(id, key, bytes.NewReader(data)); err != nil {
			t.Error(err)
		}

		// --- HAS ---
		if ok := s.Has(id, key); !ok {
			t.Errorf("expected to have key %s", key)
		}

		// --- READ ---
		_, r, err := s.Read(id, key)
		if err != nil {
			t.Error(err)
		}

		// Đọc toàn bộ bytes từ reader
		b, _ := ioutil.ReadAll(r)
		if string(b) != string(data) {
			t.Errorf("want %s have %s", data, b)
		}

		// --- DELETE ---
		if err := s.Delete(id, key); err != nil {
			t.Error(err)
		}

		// --- HAS (sau khi xóa) ---
		if ok := s.Has(id, key); ok {
			t.Errorf("expected to NOT have key %s", key)
		}
	}
}

////////////////////////////////////////////////////////////////////////////////
//                              HELPER FUNCTIONS                              //
////////////////////////////////////////////////////////////////////////////////

// newStore tạo store mới với CASPathTransformFunc để quản lý đường dẫn file.
func newStore() *Store {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}
	return NewStore(opts)
}

// teardown xóa sạch toàn bộ dữ liệu trong Store sau mỗi test.
// Đây là best practice để test độc lập, không bị "dính" dữ liệu cũ.
func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Error(err)
	}
}
