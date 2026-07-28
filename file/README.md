# file

文件上传的校验 + 落盘封装。

只做两件事：**校验**（大小、扩展名）和**保存**。不负责云存储上传、不负责图片压缩。

## 使用

```go
import "earwind.top/forge/file"

// 初始化（启动时做一次）
uploader := file.New(file.Config{
    MaxSize:   10, // 10MB
    SavePath:  "uploads",
    AllowExts: []string{".md", ".txt", ".pdf", ".doc", ".docx"},
})

// 在 handler 里调用
func CreateDoc(w http.ResponseWriter, r *http.Request) {
    f, header, err := r.FormFile("file")
    if err != nil {
        // 处理错误
    }
    defer f.Close()

    path, err := uploader.Save(f, header.Size, header.Filename)
    // path = "uploads/1706428800000000000_readme.md"
}
```

## 为什么用结构体而非全局配置

同 [`jwt`](../jwt/) 包的理由：每个服务可能配置不同的上传目录和大小限制，结构体方式更加灵活。
