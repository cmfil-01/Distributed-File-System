package p2p

// Định nghĩa các hằng số (constant) để đánh dấu loại dữ liệu nhận được.
const (
	IncomingMessage = 0x1 // 0x1 (số hexa) nghĩa là đây là một "message" bình thường
	IncomingStream  = 0x2 // 0x2 nghĩa là đây là một "stream" (luồng dữ liệu liên tục)
)

// RPC = Remote Procedure Call (lời gọi thủ tục từ xa).
// Trong project này, RPC chính là "gói tin" dùng để trao đổi dữ liệu giữa các node.
// Mỗi lần gửi dữ liệu qua mạng (transport), nó sẽ được gói trong một RPC.
type RPC struct {
	From    string // địa chỉ của peer gửi message này (ví dụ: "127.0.0.1:3000")
	Payload []byte // dữ liệu thực sự được gửi (nội dung message)
	Stream  bool   // nếu true -> đây là stream (luồng), nếu false -> message thường
}
