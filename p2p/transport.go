package p2p

import "net"

// Peer là giao diện đại diện cho "một nút từ xa" (remote node) đang kết nối với chúng ta.
// Lưu ý: nó "nhúng" (embed) luôn net.Conn, nên mọi phương thức của net.Conn đều dùng được:
//   - Read, Write, Close, LocalAddr, RemoteAddr, SetDeadline, ...
// Bên cạnh đó, Peer bổ sung 2 hàm tiện ích cho P2P:
//   - Send([]byte) error  : gửi dữ liệu thô ra kết nối một cách thống nhất
//   - CloseStream()       : thông báo kết thúc một luồng (stream) dài đang mở
type Peer interface {
	net.Conn           // kế thừa toàn bộ API của kết nối TCP/UDP/... từ Go
	Send([]byte) error // gửi dữ liệu tới peer
	CloseStream()      // báo hiệu "đóng stream" (phục vụ cơ chế stream-control)
}

// Transport là giao diện trừu tượng hóa "lớp giao tiếp mạng" giữa các node.
// Mục tiêu: tách biệt network I/O khỏi logic ứng dụng.
// Bạn có thể implement nhiều loại transport khác nhau (TCP, UDP, WebSocket, QUIC...)
// miễn là tuân theo bộ hàm dưới đây.
type Transport interface {
	// Addr trả về địa chỉ listen (ví dụ ":3000")
	Addr() string

	// Dial thiết lập kết nối tới một địa chỉ peer (ví dụ "127.0.0.1:3000")
	Dial(string) error

	// ListenAndAccept mở cổng và bắt đầu chấp nhận (accept) các kết nối đến
	ListenAndAccept() error

	// Consume trả về một kênh chỉ-đọc chứa các RPC nhận được từ peers.
	// Ứng dụng (Server) chỉ cần range/for trên kênh này để xử lý message.
	Consume() <-chan RPC

	// Close đóng transport (thường là đóng listener, giải phóng tài nguyên)
	Close() error
}
