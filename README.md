# 📦 DistributedFileStorage (Go P2P File Server)

## 🚀 Giới thiệu
Đây là dự án thử nghiệm **hệ thống lưu trữ phân tán** viết bằng **Golang**.  
Mỗi node trong mạng có thể:
- Mở port TCP để **lắng nghe kết nối từ peer khác**.  
- **Dial** (chủ động kết nối) tới các peer có sẵn.  
- **Trao đổi RPC (message/stream)** để gửi – nhận dữ liệu.  
- **Lưu trữ file cục bộ** theo cơ chế CAS (*Content Addressed Storage*) – tên file được xác định bằng hash nội dung.  

👉 Tóm lại: mỗi node là **một kho hàng trong mạng P2P**, có thể tự lưu file và chia sẻ với kho khác.

---

## ⚙️ Cấu trúc chính

```
DistributedFileStorage/
 ├── main.go                # Entry point: khởi động FileServer
 ├── server.go              # Server quản lý vòng đời node
 ├── store.go               # Store: quản lý lưu trữ file theo CAS
 ├── crypto.go              # Hàm mã hóa/giải mã, chữ ký
 ├── p2p/                   # Lớp giao tiếp P2P
 │   ├── transport.go       # Định nghĩa Peer & Transport interface
 │   ├── tcp_transport.go   # Hiện thực Transport bằng TCP
 │   ├── encoding.go        # Decoder: chuyển bytes -> RPC
 │   ├── handshake.go       # Handshake function (NOP hoặc custom)
 │   ├── message.go         # Định nghĩa RPC (From, Payload, Stream)
 │   └── tcp_transport_test.go
 ├── Makefile               # Lệnh build/test
 ├── go.mod / go.sum        # Module Go
 └── README.md
```

---

## 🔑 Thành phần chính

### P2P Layer
- **Transport**: interface trừu tượng hóa kênh liên lạc giữa các node.  
- **TCPTransport**: implementation dùng TCP.  
- **RPC**: message truyền qua mạng.  
- **Handshake**: bước bắt tay, có thể cấy logic xác thực (public key, version…).  

### Application Layer
- **FileServer**: node chính, quản lý peers và store.  
- **Store**: lớp lưu file, lưu dưới dạng hash (SHA-1 → thư mục lồng nhau).  
- **Crypto**: mã hóa/giải mã dữ liệu, bảo mật khi lưu/trao đổi.  

---

## ▶️ Cách chạy

### Yêu cầu
- Go 1.19+ (khuyến nghị)
- Git

### Build
```bash
make build
```
File thực thi sẽ nằm trong `bin/`.

### Chạy 2–3 node demo trên cùng máy
**Terminal 1:**
```bash
go run . -listen :3000
```

**Terminal 2:**
```bash
go run . -listen :7000
```

**Terminal 3 (bootstrap vào 2 node trên):**
```bash
go run . -listen :5000 -join :3000,:7000
```

Khi đó bạn có 3 node kết nối thành mạng nhỏ. Node :5000 sẽ tự dial sang :3000 và :7000.

---

## 📂 Cơ chế lưu trữ (Store)

- File được lưu trong `Root/<node_id>/<hashed_path>/<filename>`.  
- `hashed_path` = chuỗi hash SHA-1 chia thành các thư mục con 5 ký tự.  
- `filename` = full SHA-1 hash → đảm bảo duy nhất.  

Ví dụ với key `"hello"`:  
```
ggnetwork/
 └── 3000/
     └── aaf4c/
         └── 61ddc/
             └── 5c23f/...
```

---

## 🧪 Test
Chạy test của P2P:
```bash
cd p2p
go test -v
```

---

## 🛠️ Ghi chú phát triển
- `DefaultDecoder` hiện tại **ăn mất byte đầu tiên** nếu không phải stream → khuyến nghị viết decoder mới theo format `[type|length|payload]`.  
- Hash mặc định SHA-1 (demo), trong thực tế nên nâng lên **SHA-256**.  
- `Delete()` hiện xóa cả nhánh folder con, nên cẩn thận khi triển khai thật.  

---

## 📌 Kế hoạch mở rộng
- Thêm **Discovery service** (tự tìm peer thay vì hardcode bootstrap).  
- Hỗ trợ **protocol encoding** khác (JSON, Protobuf).  
- Tích hợp **consensus/quorum** để replicate file an toàn.  
- Thêm **REST API** để người dùng upload/download file dễ dàng.  
