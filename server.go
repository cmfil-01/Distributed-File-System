package main

import (
	"DistributedFileStorage/p2p"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
//                         CẤU HÌNH & KHỞI TẠO SERVER                          //
////////////////////////////////////////////////////////////////////////////////

// FileServerOpts gom toàn bộ tham số cấu hình để tạo 1 FileServer (1 node P2P).
type FileServerOpts struct {
	ID                string            // ID duy nhất cho node. Nếu rỗng sẽ tự generate (random).
	EncKey            []byte            // Khóa đối xứng để mã hóa/giải mã dữ liệu (AES-CTR ở file crypto).
	StorageRoot       string            // Thư mục gốc trên đĩa để lưu dữ liệu (mỗi node 1 “kho riêng”).
	PathTransformFunc PathTransformFunc // Hàm chuyển key -> path (ví dụ CASPathTransformFunc: băm SHA-1 chia folder).
	Transport         p2p.Transport     // Lớp giao tiếp mạng (ở đây là TCPTransport).
	BootstrapNodes    []string          // Danh sách địa chỉ peers để dial ngay khi start (kết nối vào mạng).
}

// FileServer là “node ứng dụng” thực sự:
// - Quản lý danh sách peers đang kết nối.
// - Lưu/đọc dữ liệu cục bộ qua Store.
// - Gửi/nhận message (RPC) & stream file qua Transport.
type FileServer struct {
	FileServerOpts // “embed” options → có thể truy cập trực tiếp (s.ID, s.Transport, ...)

	// ---- Trạng thái runtime được bảo vệ đồng bộ ----
	peerLock sync.Mutex          // Mutex bảo vệ map peers khi có concurrent read/write (OnPeer vs broadcast/handle).
	peers    map[string]p2p.Peer // Danh sách peers: key = peer.RemoteAddr().String(), value = kết nối (Peer).

	store  *Store        // Store cục bộ (ghi/đọc file theo PathTransformFunc).
	quitch chan struct{} // Kênh “tín hiệu dừng” server (close(quitch) để shutdown loop).
}

// NewFileServer khởi tạo 1 node FileServer với opts.
// Thiết lập Store, tạo channel quitch, map peers…
// Nếu thiếu ID thì tự sinh (generateID dùng crypto/rand).
func NewFileServer(opts FileServerOpts) *FileServer {
	if len(opts.ID) == 0 {
		opts.ID = generateID()
	}

	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}

	return &FileServer{
		FileServerOpts: opts,
		store:          NewStore(storeOpts),
		quitch:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

////////////////////////////////////////////////////////////////////////////////
//                              ĐỊNH NGHĨA MESSAGE                             //
////////////////////////////////////////////////////////////////////////////////

// Message là “phong bì” gói Payload (any) để gob encode/decode gửi qua mạng.
// Chú ý: mọi type cụ thể dùng trong Payload cần được gob.Register trong init().
type Message struct {
	Payload any
}

// Thông điệp “hãy lưu file này” (metadata, không kèm bytes file).
// - ID: ID của node phát tán (để peers quyết định lưu vào không gian nào).
// - Key: key (ở code hiện tại đang hash MD5(key gốc) trước khi đi vào CAS). Có thể xem là “định danh nội dung”.
// - Size: tổng số byte sẽ gửi qua stream (ở đây size+16 để tính thêm IV 16B của AES-CTR).
type MessageStoreFile struct {
	ID   string
	Key  string
	Size int64
}

// Thông điệp “mình cần file này” (request).
type MessageGetFile struct {
	ID  string
	Key string
}

////////////////////////////////////////////////////////////////////////////////
//                            GỬI MESSAGE ĐẾN PEERS                           //
////////////////////////////////////////////////////////////////////////////////

// broadcast encode msg bằng gob rồi gửi đến TẤT CẢ peers.
// ⚠️ CHÚ Ý RACE: s.peers là map; OnPeer có thể thêm peer đồng thời.
// Tốt nhất: giữ lock khi duyệt (hoặc copy ra slice trước), tránh concurrent map read/write.
func (s *FileServer) broadcast(msg *Message) error {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	// (Có thể lock để tránh race; ở đây giữ nguyên logic gốc)
	for _, peer := range s.peers {
		// Gửi 1 byte “IncomingMessage” để DefaultDecoder hiểu đây là message (không phải stream).
		peer.Send([]byte{p2p.IncomingMessage})
		// Tiếp theo là bytes gob.
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
//                        PUBLIC API: GET (TẢI FILE VỀ)                        //
////////////////////////////////////////////////////////////////////////////////

// Get trả về io.Reader để đọc file theo key.
// Quy trình:
// 1) Nếu đã có local → mở từ đĩa trả về ngay.
// 2) Nếu chưa có → broadcast MessageGetFile tới peers.
// 3) Chờ peers nào có file sẽ stream về: [IncomingStream][int64 fileSize][file bytes].
// 4) Ghi (giải mã) vào store cục bộ; trả về reader đọc từ disk.
//
// ⚠️ LƯU Ý THIẾT KẾ:
//   - Hiện tại code “đi vòng” TẤT CẢ peers và cố đọc fileSize. Peers nào KHÔNG trả stream sẽ không có bytes,
//     thao tác binary.Read(peer, ...) có thể BLOCK. Đây là điểm đơn giản hóa/dễ treo.
//   - Giải pháp tốt hơn: có cơ chế “đánh dấu” peer nào đã gửi IncomingStream (ví dụ qua channel) rồi CHỈ đọc từ những peer đó.
func (s *FileServer) Get(key string) (io.Reader, error) {
	// 1) Có local → dùng luôn
	if s.store.Has(s.ID, key) {
		fmt.Printf("[%s] serving file (%s) from local disk\n", s.Transport.Addr(), key)
		_, r, err := s.store.Read(s.ID, key)
		return r, err
	}

	// 2) Không có local → hỏi mạng
	fmt.Printf("[%s] dont have file (%s) locally, fetching from network...\n", s.Transport.Addr(), key)

	msg := Message{
		Payload: MessageGetFile{
			ID:  s.ID,
			Key: hashKey(key), // NOTE: đang hash MD5 trước khi đi vào CAS - điều này là thừa (CAS đã hash), nhưng vẫn OK vì “key” chỉ là định danh.
		},
	}
	if err := s.broadcast(&msg); err != nil {
		return nil, err
	}

	// Tạm “ngủ” để peers có thời gian xử lý và bắt đầu stream.
	// Trong thiết kế thực tế: dùng ack/response, timeout/ctx thay vì sleep.
	time.Sleep(time.Millisecond * 500)

	// 3) Thử đọc stream từ tất cả peers (đơn giản; dễ block nếu peer không stream)
	for _, peer := range s.peers {
		// Đọc kích thước file (int64, little-endian) để biết cần đọc bao nhiêu bytes tiếp theo.
		// Nếu peer không stream thì lệnh này có thể BLOCK (cần cải tiến như đã note).
		var fileSize int64
		binary.Read(peer, binary.LittleEndian, &fileSize)

		// Đọc đúng fileSize bytes từ peer và ghi (có giải mã AES-CTR) vào store cục bộ.
		n, err := s.store.WriteDecrypt(s.EncKey, s.ID, key, io.LimitReader(peer, fileSize))
		if err != nil {
			return nil, err
		}

		fmt.Printf("[%s] received (%d) bytes over the network from (%s)\n", s.Transport.Addr(), n, peer.RemoteAddr())

		// Báo transport: stream đã xong → read loop bên dưới sẽ tiếp tục.
		peer.CloseStream()
	}

	// 4) Trả về reader đọc từ disk (đã có sau khi ghi)
	_, r, err := s.store.Read(s.ID, key)
	return r, err
}

////////////////////////////////////////////////////////////////////////////////
//                      PUBLIC API: STORE (LƯU & PHÁT TÁN)                     //
////////////////////////////////////////////////////////////////////////////////

// Store lưu file “key” vào local, sau đó broadcast cho peers
// rồi stream nội dung thật sự (đã mã hóa) đến họ.
//
// Lưu ý: dùng TeeReader để vừa ghi local vừa giữ bản copy (fileBuffer)
// để lát nữa stream ra mạng, không cần đọc lại từ nguồn.
func (s *FileServer) Store(key string, r io.Reader) error {
	// TeeReader: đọc từ r → ghi song song vào fileBuffer (để dùng stream ra mạng).
	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	// 1) Ghi vào local store (không mã hóa ở đây; mã hóa khi stream ra mạng)
	size, err := s.store.Write(s.ID, key, tee)
	if err != nil {
		return err
	}

	// 2) Thông báo metadata cho peers: “mình có file mới”
	// Size + 16 vì khi stream AES-CTR sẽ prepend IV 16B → tổng bytes đọc/ghi ở phía nhận tăng thêm 16.
	msg := Message{
		Payload: MessageStoreFile{
			ID:   s.ID,
			Key:  hashKey(key), // như trên: hash MD5 trước CAS là thừa, nhưng vẫn là 1 key hợp lệ.
			Size: size + 16,
		},
	}
	if err := s.broadcast(&msg); err != nil {
		return err
	}

	// Cho peers thời gian xử lý message metadata (đơn giản).
	time.Sleep(time.Millisecond * 5)

	// 3) Stream dữ liệu thật sự: gửi byte flag IncomingStream, rồi mã hóa AES-CTR và đẩy ra TẤT CẢ peers.
	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	mw := io.MultiWriter(peers...)       // ghi 1 lần ra nhiều peer
	mw.Write([]byte{p2p.IncomingStream}) // byte cờ để transport “tạm dừng read-loop” và nhường việc đọc cho ứng dụng

	// copyEncrypt: prepend IV(16B) + ciphertext(=len(plain))
	n, err := copyEncrypt(s.EncKey, fileBuffer, mw)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] received and written (%d) bytes to disk\n", s.Transport.Addr(), n)
	return nil
}

////////////////////////////////////////////////////////////////////////////////
//                        QUẢN LÝ VÒNG ĐỜI & PEERS                             //
////////////////////////////////////////////////////////////////////////////////

// Stop báo cho server dừng (close channel), loop sẽ thoát.
func (s *FileServer) Stop() {
	close(s.quitch)
}

// OnPeer được gọi khi transport chấp nhận 1 peer mới.
// Thêm peer vào map (dưới lock) để các API khác (broadcast/Store/Get) có thể sử dụng.
func (s *FileServer) OnPeer(p p2p.Peer) error {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()

	s.peers[p.RemoteAddr().String()] = p
	log.Printf("connected with remote %s", p.RemoteAddr())
	return nil
}

// loop là “trái tim” của server: chờ dữ liệu từ Transport.Consume()
// - Nếu nhận được RPC message: decode gob → gọi handleMessage.
// - Nếu nhận tín hiệu dừng (quitch): đóng transport & thoát.
func (s *FileServer) loop() {
	defer func() {
		log.Println("file server stopped due to error or user quit action")
		s.Transport.Close()
	}()

	for {
		select {
		case rpc := <-s.Transport.Consume():
			// rpc.Payload là bytes gob (vì broadcast đã gửi IncomingMessage + gob)
			var msg Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Println("decoding error: ", err)
			}
			if err := s.handleMessage(rpc.From, &msg); err != nil {
				log.Println("handle message error: ", err)
			}

		case <-s.quitch:
			return
		}
	}
}

// handleMessage phân loại message theo kiểu payload (đã được gob.Register)
// và chuyển cho handler tương ứng.
func (s *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return s.handleMessageStoreFile(from, v)
	case MessageGetFile:
		return s.handleMessageGetFile(from, v)
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
//                           HANDLERS CHO MESSAGE                              //
////////////////////////////////////////////////////////////////////////////////

// handleMessageGetFile: nhận yêu cầu “hãy gửi file này cho mình” từ peer `from`.
// Nếu có file trong local store:
//   - Gửi byte IncomingStream → để peer kia “vào chế độ stream”.
//   - Gửi fileSize (int64 LE) → cho bên kia biết đọc bao nhiêu byte.
//   - Gửi bytes file (không mã hóa ở đây — CHÚ Ý: không đồng nhất với Store(), nơi ta mã hóa khi phát tán).
//     → Nếu muốn đồng bộ bảo mật, có thể mã hóa cả chiều GET này, hoặc dùng AEAD (AES-GCM).
func (s *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !s.store.Has(msg.ID, msg.Key) {
		// Không có file → log thông tin để debug.
		return fmt.Errorf("[%s] need to serve file (%s) but it does not exist on disk", s.Transport.Addr(), msg.Key)
	}

	fmt.Printf("[%s] serving file (%s) over the network\n", s.Transport.Addr(), msg.Key)

	fileSize, r, err := s.store.Read(msg.ID, msg.Key)
	if err != nil {
		return err
	}
	// Đảm bảo đóng file nếu r là ReadCloser
	if rc, ok := r.(io.ReadCloser); ok {
		defer rc.Close()
	}

	// Tìm peer đích để gửi
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer %s not in map", from)
	}

	// 1) báo IncomingStream để bên kia pause read-loop
	peer.Send([]byte{p2p.IncomingStream})
	// 2) gửi trước fileSize (LE int64) để bên kia LimitReader cho đúng số byte
	binary.Write(peer, binary.LittleEndian, fileSize)
	// 3) gửi bytes file
	n, err := io.Copy(peer, r)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] written (%d) bytes over the network to %s\n", s.Transport.Addr(), n, from)
	return nil
}

// handleMessageStoreFile: khi peer khác thông báo “mình chuẩn bị stream 1 file cỡ Size cho bạn”,
// ta đọc đúng Size byte từ kết nối peer và ghi vào store.
// ⚠️ Ở nhánh Store (push) phía bạn đã MÃ HÓA khi stream (copyEncrypt) → ở đây ghi RAW (không decrypt).
//
//	Trong code này, nhánh “lắng nghe push” không decrypt (khác với nhánh Get() dùng WriteDecrypt).
//	Bạn có thể điều chỉnh để đồng nhất (decrypt ở đây), hoặc chỉ mã hóa trên đường truyền (không mã hóa lưu trữ).
func (s *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer (%s) could not be found in the peer list", from)
	}

	// Ghi đúng msg.Size bytes từ peer vào store.
	// (Nếu muốn decrypt khi ghi, hãy dùng WriteDecrypt với key tương ứng.)
	n, err := s.store.Write(msg.ID, msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}

	fmt.Printf("[%s] written %d bytes to disk\n", s.Transport.Addr(), n)

	// Báo transport: stream đã hoàn tất → cho read-loop tiếp tục chạy.
	peer.CloseStream()
	return nil
}

////////////////////////////////////////////////////////////////////////////////
//                             KHỞI ĐỘNG / KẾT NỐI                             //
////////////////////////////////////////////////////////////////////////////////

// bootstrapNetwork: thử dial đến tất cả bootstrap nodes (nếu có).
// Chạy mỗi dial trong 1 goroutine để không chặn luồng chính.
func (s *FileServer) bootstrapNetwork() error {
	for _, addr := range s.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}
		go func(addr string) {
			fmt.Printf("[%s] attemping to connect with remote %s\n", s.Transport.Addr(), addr)
			if err := s.Transport.Dial(addr); err != nil {
				log.Println("dial error: ", err)
			}
		}(addr)
	}
	return nil
}

// Start: entrypoint của FileServer.
// - ListenAndAccept: mở cổng, chấp nhận kết nối.
// - bootstrapNetwork: dial vào peers khởi động.
// - loop: bắt đầu tiêu thụ message RPC.
func (s *FileServer) Start() error {
	fmt.Printf("[%s] starting fileserver...\n", s.Transport.Addr())

	if err := s.Transport.ListenAndAccept(); err != nil {
		return err
	}
	s.bootstrapNetwork()
	s.loop()
	return nil
}

////////////////////////////////////////////////////////////////////////////////
//                       GOB: ĐĂNG KÝ KIỂU PAYLOAD (IMPORTANT)                 //
////////////////////////////////////////////////////////////////////////////////

// init đăng ký các kiểu payload để gob có thể encode/decode chính xác.
// Nếu quên đăng ký, gob sẽ không biết cách giải mã “any” bên trong Message.Payload.
func init() {
	gob.Register(MessageStoreFile{})
	gob.Register(MessageGetFile{})
}
