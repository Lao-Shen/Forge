# peicon

从 Windows PE 文件（exe、dll、mui、mun）中提取图标。

只做一件事：**读图标**。不写回 PE、不做图标编辑、不处理 apk/ipa/icns。

## 使用

```go
import "earwind.top/forge/peicon"
```

### 导出全部图标

```go
files, err := peicon.ExtractAll(`D:\Install\Quark\quark.exe`, "out")
// files 是实际写出的文件路径
```

产物按图标组分目录，每组一个多尺寸 `.ico` 加各尺寸的 PNG：

```
out/
├── 0_IDR_MAINFRAME/
│   ├── icon.ico          该组全部 20 个尺寸
│   ├── 256x256_id1.png   同尺寸有多张时才加 _id 后缀
│   ├── 256x256_id6.png
│   ├── 128x128.png       尺寸唯一时用简洁名
│   └── ...
└── 1_IDR_X001_APP_LIST/
    ├── icon.ico
    └── 256x256.png
```

### 按尺寸导出单张

```go
err := peicon.ExtractSize(`D:\Install\Quark\quark.exe`, 256, 256, "app.png")
```

跨全部图标组挑选最匹配的一张，缩放到**恰好** 256×256 存为 PNG。挑选顺序：

1. 尺寸完全相等
2. 两个维度都不小于目标（只缩不放，画质优先）
3. 其余（需要放大）

同级内取尺寸差最小者，再取色深最高者。

### 只枚举不导出

```go
groups, err := peicon.List(exePath)
for _, g := range groups {
    for _, im := range g.Images {
        fmt.Println(g.Name, im.ResID, im.Width, im.Height, im.BitDepth, im.Format)
    }
}
```

`List` 只读头部不解码像素，很快。`Image.Data` 是原始资源字节，可自行处理。

## 命令行

[`cmd/peicon`](../cmd/peicon/) 是个可直接跑的示例，覆盖上面三种用法：

```bash
go run ./cmd/peicon -in app.exe                     # 导出全部 -> app_icons/
go run ./cmd/peicon -in app.exe -out icons          # 导出全部 -> icons/
go run ./cmd/peicon -in app.exe -size 256           # 单张 -> app_256x256.png
go run ./cmd/peicon -in app.exe -size 128x64 -out a.png
go run ./cmd/peicon -in app.exe -list               # 只列出，不写文件
```

`-size` 支持 `256`（方形简写）和 `256x128` 两种写法。`-out` 留空时按输入文件名自动推导。

```
$ go run ./cmd/peicon -in C:\Windows\System32\winver.exe -list
组 #0  1  (13 张)
    id=1      48x48   4bpp  dib
    id=2      32x32   4bpp  dib
    ...
    id=9     256x256 32bpp  png
    id=13     16x16  32bpp  dib

共 1 组 / 13 张
```

## 注意事项

**没有图标是常见情况，不是错误。** Win10 1903 起，`notepad.exe`、`imageres.dll`、`shell32.dll` 这些系统文件的图标资源已迁移到 `C:\Windows\SystemResources\*.mun`。实测这三个文件都是 0 图标，图标在同名的 `.mun` 里：

```go
peicon.List(`C:\Windows\System32\imageres.dll`)              // 0 组
peicon.List(`C:\Windows\SystemResources\imageres.dll.mun`)   // 337 组 / 2707 张
```

`List` 对这种情况返回空切片 + `nil`（查询接口，空是合法答案）；`ExtractAll` / `ExtractSize` 返回明确错误。

**exe 内部没有「ico 图标」和「png 图标」之分。** PE 里存的是 RT_ICON 资源，每张图像要么是 DIB 位图，要么是 PNG 流（Vista 起 256×256 常用），同一个组里两种可以混存。`Image.Format` 会告诉你是哪种，但通常不需要关心 —— 导出 PNG 时两种都会被正确解码。

**`.ico` 产物是原始载荷逐字节透传**，不做重编码，所以画质无损、体积与 PE 内一致。PNG 产物则是解码后重新编码的。

**单张图像解码失败只跳过该 PNG**，不中断整体导出 —— 它的原始数据仍完整保留在 `icon.ico` 里。

**不支持的编码**：16bpp、RLE 压缩、BI_BITFIELDS。扫过 2882 张真实图标（Quark + imageres.dll.mun）这些出现 0 次，遇到会返回明确错误而不是静默出错。

## 为什么不直接依赖 orcastor/fico

[fico](https://github.com/orcastor/fico) 是个好库，本包的 `.rsrc` 资源树遍历与 PE 图标结构体就是从它抄的（MIT，见 `rsrc.go` 文件头）。但没有整体依赖它，原因有二：

1. **依赖成本不成比例**。fico 为覆盖 apk/ipa/icns/jpeg2000 拖进 9 个传递依赖，而 PE 这条路径实际只用到 stdlib + `x/image`。抄那几十行 = 同样的代码、零新依赖。
2. **它的 DIB 解码有 bug**，产物有可见瑕疵，实测验证过两个：
   - 六个位深分支的循环都写成 `for yy := h-1; yy > 0; yy--`。DIB 自底向上存储，`yy==0` 分支体永不执行，**每张图丢顶部一行**。
   - 掩码/调色板偏移按 `w*w<<2`、`w>>3` 计算，假定图标为正方形且行无填充。实际每行按 4 字节对齐：`stride = ((w*bpp+31)/32)*4`。AND 掩码是 1bpp，16 宽的掩码 stride 应为 4 而非 `16>>3`=2，48 宽应为 8 而非 6 —— 常见宽度全中，**依赖掩码表达透明的老图标（4/8bpp）边缘会碎成锯齿**。

所以本包的 DIB 解码改为：1/4/8/24bpp 交给 `golang.org/x/image/bmp`（维护中、stride 正确、与已有的 `x/image/draw` 同模块，不增加依赖），只有 32bpp 自己处理 —— 因为 `bmp.Decode` 对 40 字节头会把 alpha 强制刷成 0xFF（其 `allowAlpha = infoLen > infoHeaderLen`，源码注释写明不支持 ICO），而 32bpp 恰好是最简单的一档：BGRA、无调色板、无位拆分。

同一张 Quark 16×16 8bpp 图标的透明度轮廓对比（`#` 不透明）：

```
      fico              peicon
................    ......####......    ← fico 丢掉的顶行
################    ....########....
################    ..############..
################    .##############.
################    .##############.
################    .##############.
################    ################
.##############.    ################
################    ################
.##############.    ################    ← fico 隔行正确/错误交替
################    ################
..############..    .##############.
################    .##############.
...##########...    ..############..
################    ...##########...
.....######.....    .....######.....
```
