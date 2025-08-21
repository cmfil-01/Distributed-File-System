package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTCPTransport là unit test để kiểm tra xem TCPTransport có khởi tạo đúng
// và có thể bắt đầu Listen (mở cổng) thành công hay không.
func TestTCPTransport(t *testing.T) {
	// B1. Tạo options cho TCPTransport
	opts := TCPTransportOpts{
		ListenAddr:    ":3000",          // node sẽ lắng nghe ở port 3000
		HandshakeFunc: NOPHandshakeFunc, // không bắt tay gì, luôn chấp nhận peer
		Decoder:       DefaultDecoder{}, // dùng decoder mặc định (đọc byte thô)
	}

	// B2. Khởi tạo transport với options trên
	tr := NewTCPTransport(opts)

	// B3. Kiểm tra giá trị ListenAddr của transport có đúng như mong đợi không
	assert.Equal(t, tr.ListenAddr, ":3000")

	// B4. Gọi ListenAndAccept() để bắt đầu lắng nghe cổng TCP
	// assert.Nil kiểm tra kết quả trả về là nil (tức là không có lỗi)
	assert.Nil(t, tr.ListenAndAccept())
}
