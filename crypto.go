package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
)

// generateID tạo một chuỗi ID ngẫu nhiên 32 byte (256 bit)
// và chuyển thành dạng chuỗi hex (64 ký tự).
// Thường dùng để sinh ID duy nhất cho file, peer, hay phiên giao dịch.
func generateID() string {
	buf := make([]byte, 32)        // tạo buffer 32 byte
	io.ReadFull(rand.Reader, buf)  // sinh ngẫu nhiên 32 byte an toàn mật mã
	return hex.EncodeToString(buf) // chuyển thành chuỗi hex
}

// hashKey băm một chuỗi bằng MD5 và trả về kết quả hex.
// ⚠️ Lưu ý: MD5 KHÔNG an toàn cho mật mã, chỉ dùng để "đánh dấu nhanh"
// hoặc tạo key ngắn gọn, không dùng cho bảo mật thực sự.
func hashKey(key string) string {
	hash := md5.Sum([]byte(key))       // băm chuỗi thành 16 byte
	return hex.EncodeToString(hash[:]) // chuyển sang hex (32 ký tự)
}

// newEncryptionKey sinh một khóa AES ngẫu nhiên 32 byte (AES-256).
// Mỗi lần gọi sẽ tạo một khóa khác nhau.
func newEncryptionKey() []byte {
	keyBuf := make([]byte, 32)       // buffer 32 byte
	io.ReadFull(rand.Reader, keyBuf) // đọc ngẫu nhiên 32 byte từ hệ thống
	return keyBuf
}

// copyStream là hàm "ống dẫn" dùng chung cho mã hóa và giải mã dữ liệu.
// - stream: đối tượng cipher.Stream (AES-CTR) để thực hiện XORKeyStream.
// - blockSize: kích thước khối (AES = 16 byte).
// - src: nguồn dữ liệu (ví dụ file hoặc buffer).
// - dst: nơi ghi dữ liệu (ví dụ file khác, buffer).
// Hoạt động: đọc từng chunk từ src, mã hóa/giải mã tại chỗ, rồi ghi sang dst.
// Trả về tổng số byte đã xử lý (tính cả blockSize để cộng thêm phần IV).
func copyStream(stream cipher.Stream, blockSize int, src io.Reader, dst io.Writer) (int, error) {
	var (
		buf = make([]byte, 32*1024) // buffer tạm 32KB
		nw  = blockSize             // tổng byte đã xử lý, khởi đầu = blockSize (đếm cả IV)
	)
	for {
		// đọc dữ liệu từ src vào buf
		n, err := src.Read(buf)

		if n > 0 {
			// mã hóa/giải mã tại chỗ trên buf[:n]
			stream.XORKeyStream(buf, buf[:n])

			// ghi dữ liệu kết quả ra dst
			nn, err := dst.Write(buf[:n])
			if err != nil {
				return 0, err
			}
			nw += nn
		}

		// EOF = đã đọc hết dữ liệu → dừng vòng lặp
		if err == io.EOF {
			break
		}

		// lỗi khác → trả về
		if err != nil {
			return 0, err
		}
	}
	return nw, nil
}

// copyDecrypt giải mã dữ liệu AES-CTR.
// Đầu vào src có format: [IV(16 byte)][ciphertext...]
// Bước 1: Tạo AES block cipher từ key.
// Bước 2: Đọc 16 byte IV từ src.
// Bước 3: Tạo stream CTR từ block + IV.
// Bước 4: Gọi copyStream để giải mã phần còn lại sang dst.
func copyDecrypt(key []byte, src io.Reader, dst io.Writer) (int, error) {
	block, err := aes.NewCipher(key) // tạo AES cipher từ key
	if err != nil {
		return 0, err
	}

	// Đọc IV 16 byte từ src
	iv := make([]byte, block.BlockSize())
	if _, err := src.Read(iv); err != nil {
		return 0, err
	}

	// Tạo CTR stream với key + IV
	stream := cipher.NewCTR(block, iv)

	// Giải mã dữ liệu còn lại
	return copyStream(stream, block.BlockSize(), src, dst)
}

// copyEncrypt mã hóa dữ liệu AES-CTR.
// Output format: [IV(16 byte)][ciphertext...]
// Bước 1: Tạo AES block cipher từ key.
// Bước 2: Sinh IV ngẫu nhiên 16 byte.
// Bước 3: Ghi IV vào dst (prepend).
// Bước 4: Tạo CTR stream từ block + IV.
// Bước 5: Gọi copyStream để mã hóa dữ liệu từ src sang dst.
func copyEncrypt(key []byte, src io.Reader, dst io.Writer) (int, error) {
	block, err := aes.NewCipher(key) // tạo AES cipher từ key
	if err != nil {
		return 0, err
	}

	// Tạo IV ngẫu nhiên 16 byte
	iv := make([]byte, block.BlockSize()) // blockSize = 16 cho AES
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return 0, err
	}

	// Ghi IV vào đầu output để bên nhận có thể giải mã
	if _, err := dst.Write(iv); err != nil {
		return 0, err
	}

	// Tạo CTR stream từ block + IV
	stream := cipher.NewCTR(block, iv)

	// Mã hóa dữ liệu từ src sang dst
	return copyStream(stream, block.BlockSize(), src, dst)
}
