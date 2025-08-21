package p2p

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

// TCPPeer đại diện cho một "peer" (node khác) mà ta đã kết nối TCP thành công.
type TCPPeer struct {
	// Kết nối TCP thực sự
	net.Conn

	// outbound = true nếu mình là bên chủ động Dial()
	// outbound = false nếu mình là bên được Accept() (nghe và nhận kết nối)
	outbound bool

	// WaitGroup để đồng bộ khi xử lý stream (dữ liệu liên tục).
	wg *sync.WaitGroup
}

// Hàm tạo TCPPeer mới
func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		outbound: outbound,
		wg:       &sync.WaitGroup{},
	}
}

// CloseStream báo hiệu stream đã kết thúc
// (giảm counter của WaitGroup → cho phép read loop chạy tiếp).
func (p *TCPPeer) CloseStream() {
	p.wg.Done()
}

// Send gửi dữ liệu ra TCP connection
func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Conn.Write(b)
	return err
}

// -----------------------------
// Cấu hình cho TCPTransport
// -----------------------------
type TCPTransportOpts struct {
	ListenAddr    string           // địa chỉ để listen (ví dụ ":3000")
	HandshakeFunc HandshakeFunc    // hàm bắt tay khi peer kết nối
	Decoder       Decoder          // bộ giải mã bytes → RPC
	OnPeer        func(Peer) error // callback khi có peer mới
}

// -----------------------------
// TCPTransport: hiện thực Transport bằng TCP
// -----------------------------
type TCPTransport struct {
	TCPTransportOpts              // nhúng luôn options
	listener         net.Listener // TCP listener
	rpcch            chan RPC     // channel chứa các RPC nhận được
}

// Hàm tạo TCPTransport mới
func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		rpcch:            make(chan RPC, 1024), // buffer 1024 RPC
	}
}

// Addr: trả về địa chỉ mà transport đang lắng nghe
func (t *TCPTransport) Addr() string {
	return t.ListenAddr
}

// Consume: trả về channel chỉ-đọc để đọc các RPC incoming
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcch
}

// Close: đóng listener (ngừng nhận kết nối mới)
func (t *TCPTransport) Close() error {
	return t.listener.Close()
}

// Dial: chủ động kết nối tới 1 địa chỉ TCP khác
// Khi kết nối thành công → chạy handleConn với outbound=true
func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	go t.handleConn(conn, true)

	return nil
}

// ListenAndAccept: mở cổng TCP và bắt đầu chấp nhận kết nối
func (t *TCPTransport) ListenAndAccept() error {
	var err error

	t.listener, err = net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}

	go t.startAcceptLoop()

	log.Printf("TCP transport listening on port: %s\n", t.ListenAddr)

	return nil
}

// startAcceptLoop: vòng lặp accept liên tục
func (t *TCPTransport) startAcceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			// listener đã bị đóng → dừng vòng lặp
			return
		}

		if err != nil {
			fmt.Printf("TCP accept error: %s\n", err)
		}

		// spawn goroutine để xử lý từng kết nối riêng
		go t.handleConn(conn, false)
	}
}

// handleConn: xử lý một kết nối TCP (cả inbound & outbound)
func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {
	var err error

	// Đảm bảo khi hàm kết thúc thì đóng kết nối
	defer func() {
		fmt.Printf("dropping peer connection: %s", err)
		conn.Close()
	}()

	// Tạo peer mới
	peer := NewTCPPeer(conn, outbound)

	// Bước 1: Handshake (nếu thất bại thì return ngay)
	if err = t.HandshakeFunc(peer); err != nil {
		return
	}

	// Bước 2: Gọi callback OnPeer (ví dụ: add peer vào map)
	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}

	// Bước 3: Read loop – đọc RPC liên tục
	for {
		rpc := RPC{}
		// Giải mã dữ liệu từ kết nối → RPC
		err = t.Decoder.Decode(conn, &rpc)
		if err != nil {
			// nếu lỗi đọc (EOF, timeout, v.v.) → dừng
			return
		}

		// Gắn thông tin địa chỉ nguồn
		rpc.From = conn.RemoteAddr().String()

		// Nếu là stream:
		if rpc.Stream {
			peer.wg.Add(1)
			fmt.Printf("[%s] incoming stream, waiting...\n", conn.RemoteAddr())
			// chờ đến khi CloseStream() được gọi
			peer.wg.Wait()
			fmt.Printf("[%s] stream closed, resuming read loop\n", conn.RemoteAddr())
			continue
		}

		// Nếu là message thường → đẩy vào channel
		t.rpcch <- rpc
	}
}
