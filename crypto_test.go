package main

import (
	"bytes"
	"fmt"
	"testing"
)

// TestCopyEncryptDecrypt kiểm tra quy trình:
//
//	(1) Mã hóa dữ liệu từ src -> dst
//	(2) Giải mã lại từ dst -> out
//
// và xác nhận nội dung sau giải mã đúng bằng với payload ban đầu.
func TestCopyEncryptDecrypt(t *testing.T) {
	// Payload "gốc" cần bảo vệ
	payload := "Foo not bar"

	// src: nguồn dữ liệu (reader) lấy từ payload
	src := bytes.NewReader([]byte(payload))

	// dst: đích tạm để giữ dữ liệu SAU KHI MÃ HÓA (ciphertext + overhead)
	dst := new(bytes.Buffer)

	// Tạo khóa mã hóa (chi tiết nằm trong newEncryptionKey)
	key := newEncryptionKey()

	// Thực hiện mã hóa: copyEncrypt đọc từ src, ghi ra dst
	// Thông thường, dst sẽ chứa: [nonce/iv/tag ...][ciphertext]
	_, err := copyEncrypt(key, src, dst)
	if err != nil {
		t.Error(err) // test fail nếu mã hóa lỗi
	}

	// In ra độ dài để quan sát (tùy chọn, không ảnh hưởng test)
	fmt.Println(len(payload))      // độ dài plaintext
	fmt.Println(len(dst.String())) // độ dài dữ liệu đã mã hóa (thường dài hơn)

	// out: buffer để nhận plaintext sau khi giải mã
	out := new(bytes.Buffer)

	// Giải mã: copyDecrypt đọc từ dst (ciphertext + overhead),
	// ghi plaintext ra out
	nw, err := copyDecrypt(key, dst, out)
	if err != nil {
		t.Error(err) // test fail nếu giải mã lỗi
	}

	// Kiểm tra kích thước: tùy theo cách hiện thực, test này kỳ vọng
	// copyDecrypt trả về số byte đã "xử lý" = len(payload) + 16
	// (16 byte ở đây thường là overhead: nonce/IV/tag... đi kèm với dữ liệu mã hóa).
	if nw != 16+len(payload) {
		t.Fail() // fail nếu kích thước không khớp kỳ vọng
	}

	// Nội dung sau giải mã phải đúng y hệt payload ban đầu
	if out.String() != payload {
		t.Errorf("decryption failed!!!")
	}
}
