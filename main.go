package main

import (
	"DistributedFileStorage/p2p"
	"bytes"
	"fmt"
	"io"
	"log"
	"time"
)

// makeServer là hàm tiện ích tạo ra một FileServer mới.
// Nó sẽ:
//  1. Cấu hình TCPTransport (listen, handshake, decoder).
//  2. Cấu hình FileServerOpts (key mã hóa, storage, transport, bootstrap nodes).
//  3. Khởi tạo FileServer.
//  4. Gắn hàm xử lý OnPeer (khi có peer mới kết nối).
//
// listenAddr: địa chỉ cổng mà server sẽ lắng nghe (ví dụ ":3000").
// nodes...  : danh sách địa chỉ các peer khác để bootstrap (kết nối ban đầu).
func makeServer(listenAddr string, nodes ...string) *FileServer {
	// Thiết lập transport TCP (địa chỉ listen, hàm bắt tay, bộ giải mã)
	tcptransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandshakeFunc: p2p.NOPHandshakeFunc, // handshake "no-op": chấp nhận mọi peer
		Decoder:       p2p.DefaultDecoder{}, // dùng decoder mặc định
	}
	tcpTransport := p2p.NewTCPTransport(tcptransportOpts)

	// Cấu hình FileServer
	fileServerOpts := FileServerOpts{
		EncKey:            newEncryptionKey(),      // sinh key ngẫu nhiên cho mã hóa
		StorageRoot:       listenAddr + "_network", // thư mục lưu trữ dữ liệu cục bộ
		PathTransformFunc: CASPathTransformFunc,    // cách ánh xạ key -> path
		Transport:         tcpTransport,            // lớp giao tiếp mạng
		BootstrapNodes:    nodes,                   // các peer ban đầu để kết nối
	}

	// Khởi tạo FileServer
	s := NewFileServer(fileServerOpts)

	// Khi transport có peer mới → gọi OnPeer của FileServer để quản lý
	tcpTransport.OnPeer = s.OnPeer

	return s
}

func main() {
	// Tạo 3 server (mô phỏng 3 node P2P chạy cùng máy)
	// s1 lắng nghe ở cổng :3000, không bootstrap node nào
	s1 := makeServer(":3000", "")
	// s2 lắng nghe ở cổng :7000, không bootstrap node nào
	s2 := makeServer(":7000", "")
	// s3 lắng nghe ở cổng :5000, bootstrap kết nối tới s1(:3000) và s2(:7000)
	s3 := makeServer(":5000", ":3000", ":7000")

	// Khởi động server s1 trong goroutine
	go func() { log.Fatal(s1.Start()) }()
	time.Sleep(500 * time.Millisecond) // chờ một chút để s1 khởi động

	// Khởi động server s2 trong goroutine
	go func() { log.Fatal(s2.Start()) }()
	time.Sleep(2 * time.Second) // chờ để s2 cũng sẵn sàng

	// Khởi động server s3
	// Vì s3 có bootstrap nodes nên nó sẽ tự kết nối tới s1 và s2
	go s3.Start()
	time.Sleep(2 * time.Second) // chờ để mạng P2P được hình thành

	// Mô phỏng việc lưu trữ và truy xuất dữ liệu trên s3
	for i := 0; i < 20; i++ {
		// Tạo key file giả lập
		key := fmt.Sprintf("picture_%d.png", i)

		// Data giả lập (thay vì đọc từ file, ta dùng chuỗi ngắn)
		data := bytes.NewReader([]byte("my big data file here!"))

		// Lưu file vào s3 (dữ liệu sẽ được mã hóa + lưu local, và có thể replicate ra peers)
		s3.Store(key, data)

		// Xóa file khỏi store cục bộ của s3 (giả lập tình huống mất dữ liệu cục bộ)
		if err := s3.store.Delete(s3.ID, key); err != nil {
			log.Fatal(err)
		}

		// Thử lấy lại file từ mạng (P2P replication)
		r, err := s3.Get(key)
		if err != nil {
			log.Fatal(err)
		}

		// Đọc toàn bộ dữ liệu đã lấy được
		b, err := io.ReadAll(r)
		if err != nil {
			log.Fatal(err)
		}

		// In nội dung ra màn hình
		fmt.Println(string(b))
	}
}
