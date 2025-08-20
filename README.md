# 📦 Hệ thống Tệp Phân Tán viết bằng Go

Một dự án demo về **Distributed File System (DFS)** được cài đặt bằng ngôn ngữ Go.  
Mục tiêu của project là minh họa cách xây dựng một dịch vụ lưu trữ file theo mô hình **peer-to-peer**, sử dụng kết nối TCP, cơ chế truyền thông điệp và nhân bản dữ liệu cơ bản.

---

## 🚀 Tính năng
- Viết hoàn toàn bằng **Go**.  
- Mạng ngang hàng (**P2P**) thông qua TCP.  
- Lưu trữ và truy xuất file trên nhiều node khác nhau.  
- Nhân bản (replication) cơ bản để tăng khả năng chịu lỗi.  
- Cấu trúc module rõ ràng, dễ học các khái niệm hệ phân tán.  

---

## 🛠️ Cài đặt

### Yêu cầu
- [Go](https://go.dev/dl/) **>= 1.20**  
- `git` để clone repository  

### Clone dự án
```bash
git clone https://github.com/anthdm/distributedfilesystemgo.git
cd distributedfilesystemgo
```

---

## ⚙️ Sử dụng

### Build
```bash
go build -o bin/fs
```

### Chạy
```bash
./bin/fs
```
Trên Windows:
```powershell
.\bin\fs.exe
```

### Chạy với Makefile (Linux/Mac)
```bash
make run
```

### Chạy test
```bash
go test ./... -v
```

---

## 📖 Ví dụ
Khởi chạy nhiều peer ở các terminal khác nhau:
```bash
./bin/fs --port 3000
./bin/fs --port 3001
./bin/fs --port 3002
```

Upload một file:
```bash
telnet localhost 3000
> STORE myfile.txt
```

Truy xuất file từ node khác:
```bash
telnet localhost 3001
> GET myfile.txt
```

---

## 🧩 Cấu trúc dự án
```
.
├── p2p/           # Lớp mạng ngang hàng (peer-to-peer)
├── storage/       # Xử lý lưu trữ file
├── main.go        # Điểm vào của chương trình
├── Makefile       # Các lệnh build/run nhanh
└── go.mod         # Khai báo module Go
```

---

## 🤝 Đóng góp
Mọi đóng góp đều được hoan nghênh!  
Hãy fork repo, tạo branch riêng và gửi pull request.  

---

## 📜 Giấy phép
Dự án này phát hành theo giấy phép **MIT License**.  
