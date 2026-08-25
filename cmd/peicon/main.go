// 命令行示例：演示 earwind.top/forge/peicon 的用法。
//
//	go run ./cmd/peicon -in C:\Windows\System32\winver.exe
//	go run ./cmd/peicon -in C:\Windows\System32\winver.exe -size 256
//	go run ./cmd/peicon -in C:\Windows\System32\winver.exe -list
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"earwind.top/forge/peicon"
)

func main() {
	in := flag.String("in", "", "输入的 exe/dll/mun 路径（必填）")
	out := flag.String("out", "", "输出路径；导出全部时是目录，导出单张时是文件。留空则自动推导")
	size := flag.String("size", "", "目标像素，如 256 或 256x128。留空表示导出全部图标")
	list := flag.Bool("list", false, "只列出图标信息，不导出任何文件")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "从 Windows PE 文件中提取图标。\n\n用法:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n示例:\n")
		fmt.Fprintf(os.Stderr, "  peicon -in app.exe                  导出全部图标到 app_icons/\n")
		fmt.Fprintf(os.Stderr, "  peicon -in app.exe -out icons       导出全部图标到 icons/\n")
		fmt.Fprintf(os.Stderr, "  peicon -in app.exe -size 256        导出 256x256 到 app_256x256.png\n")
		fmt.Fprintf(os.Stderr, "  peicon -in app.exe -size 64 -out a.png\n")
		fmt.Fprintf(os.Stderr, "  peicon -in app.exe -list            只看有哪些图标\n")
	}
	flag.Parse()

	if *in == "" {
		flag.Usage()
		os.Exit(2)
	}

	var err error
	switch {
	case *list:
		err = runList(*in)
	case *size != "":
		err = runOne(*in, *size, *out)
	default:
		err = runAll(*in, *out)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

// runList 只枚举，不解码像素，不写文件。
func runList(in string) error {
	groups, err := peicon.List(in)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		fmt.Printf("%s 中没有图标资源\n", in)
		fmt.Println("提示: Win10 1903 起系统文件的图标已迁移，试试 C:\\Windows\\SystemResources\\<同名>.mun")
		return nil
	}

	total := 0
	for _, g := range groups {
		fmt.Printf("组 #%d  %s  (%d 张)\n", g.Index, g.Name, len(g.Images))
		for _, im := range g.Images {
			fmt.Printf("    id=%-5d %3dx%-3d %2dbpp  %s\n",
				im.ResID, im.Width, im.Height, im.BitDepth, im.Format)
		}
		total += len(g.Images)
	}
	fmt.Printf("\n共 %d 组 / %d 张\n", len(groups), total)
	return nil
}

// runAll 导出全部图标：每组一个子目录，内含多尺寸 .ico 和各尺寸 PNG。
func runAll(in, out string) error {
	if out == "" {
		out = baseName(in) + "_icons"
	}

	files, err := peicon.ExtractAll(in, out)
	if err != nil {
		return err
	}

	ico, png := 0, 0
	for _, f := range files {
		if strings.EqualFold(filepath.Ext(f), ".ico") {
			ico++
		} else {
			png++
		}
	}
	fmt.Printf("%s -> %s\n", in, out)
	fmt.Printf("导出完成: %d 个 .ico + %d 个 .png\n", ico, png)
	return nil
}

// runOne 按目标像素导出单张 PNG。
func runOne(in, size, out string) error {
	w, h, err := parseSize(size)
	if err != nil {
		return err
	}
	if out == "" {
		out = fmt.Sprintf("%s_%dx%d.png", baseName(in), w, h)
	}

	if err := peicon.ExtractSize(in, w, h, out); err != nil {
		return err
	}
	fmt.Printf("%s -> %s (%dx%d)\n", in, out, w, h)
	return nil
}

// parseSize 解析 "256" 或 "256x128" 两种写法。
func parseSize(s string) (int, int, error) {
	parts := strings.SplitN(strings.ToLower(s), "x", 2)

	w, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || w <= 0 {
		return 0, 0, fmt.Errorf("无法解析尺寸 %q，应为 256 或 256x128 这样的格式", s)
	}
	if len(parts) == 1 {
		return w, w, nil
	}

	h, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || h <= 0 {
		return 0, 0, fmt.Errorf("无法解析尺寸 %q，应为 256 或 256x128 这样的格式", s)
	}
	return w, h, nil
}

// baseName 取不含扩展名的文件名，用于推导默认输出路径。
func baseName(p string) string {
	b := filepath.Base(p)
	return strings.TrimSuffix(b, filepath.Ext(b))
}
