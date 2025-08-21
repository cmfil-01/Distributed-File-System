package p2p

// HandshakeFunc là một "kiểu hàm" (function type).
// Nó nhận vào một Peer (đại diện cho kết nối tới một node khác)
// và trả về error.
// - Nếu trả về nil: nghĩa là handshake thành công, peer được chấp nhận.
// - Nếu trả về error: nghĩa là handshake thất bại, kết nối sẽ bị đóng.
// Ý tưởng: cho phép bạn tự định nghĩa logic "bắt tay" khi 2 node kết nối với nhau.
type HandshakeFunc func(Peer) error

// NOPHandshakeFunc là một hàm có sẵn (NOP = No Operation).
// Hàm này luôn trả về nil, tức là "luôn chấp nhận mọi kết nối, không kiểm tra gì cả".
// Nó dùng như mặc định, khi bạn không cần xác thực hay bắt tay phức tạp.
func NOPHandshakeFunc(Peer) error { return nil }
