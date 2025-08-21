package p2p

import (
	"encoding/gob"
	"io"
)

// Decoder là interface chung cho tất cả "bộ giải mã" (decoder).
// Ý tưởng: khi nhận dữ liệu thô (bytes) từ kết nối TCP,
// ta cần một bộ giải mã để biến bytes đó thành struct RPC.
type Decoder interface {
	// Decode đọc dữ liệu từ io.Reader (ví dụ: net.Conn)
	// và ghi kết quả vào con trỏ *RPC.
	// Trả về error nếu có lỗi khi đọc hoặc giải mã.
	Decode(io.Reader, *RPC) error
}

// ------------------------------
// GOBDecoder: dùng encoding/gob
// ------------------------------

// GOBDecoder là một kiểu "trống" (struct không field),
// chỉ để thỏa interface Decoder. Nó sẽ dùng gói chuẩn "encoding/gob".
type GOBDecoder struct{}

// Decode của GOBDecoder:
// 1) Tạo gob.NewDecoder từ io.Reader (kết nối).
// 2) Gọi Decode để giải mã dữ liệu gob vào struct RPC.
// Ưu điểm: có thể gửi/nhận struct phức tạp, không chỉ []byte.
func (dec GOBDecoder) Decode(r io.Reader, msg *RPC) error {
	return gob.NewDecoder(r).Decode(msg)
}

// ------------------------------
// DefaultDecoder: tự chế
// ------------------------------

// DefaultDecoder là bộ giải mã đơn giản nhất:
// đọc thẳng byte từ kết nối, không có format phức tạp.
type DefaultDecoder struct{}

// Decode của DefaultDecoder:
// - Nó đọc 1 byte đầu tiên để kiểm tra xem đây có phải "stream" không.
// - Nếu là stream, chỉ gắn cờ msg.Stream = true và return.
// - Nếu không phải stream, thì đọc tiếp dữ liệu vào Payload.
func (dec DefaultDecoder) Decode(r io.Reader, msg *RPC) error {
	// Đọc 1 byte đầu tiên từ kết nối
	peekBuf := make([]byte, 1)
	if _, err := r.Read(peekBuf); err != nil {
		// Nếu không đọc được gì (EOF chẳng hạn), trả về nil (coi như im lặng).
		return nil
	}

	// Kiểm tra xem byte đầu có phải flag "IncomingStream" không.
	// Nếu đúng: đây là một stream, không decode thêm dữ liệu,
	// chỉ gắn cờ Stream = true để logic bên ngoài xử lý.
	stream := peekBuf[0] == IncomingStream
	if stream {
		msg.Stream = true
		return nil
	}

	// Nếu không phải stream:
	// Đọc tiếp tối đa 1028 byte dữ liệu từ kết nối vào buffer.
	buf := make([]byte, 1028)
	n, err := r.Read(buf)
	if err != nil {
		// Nếu có lỗi khi đọc (kết nối đóng, timeout...), trả error ra ngoài.
		return err
	}

	// Lưu dữ liệu thực sự đọc được (buf[:n]) vào msg.Payload.
	msg.Payload = buf[:n]

	return nil
}
