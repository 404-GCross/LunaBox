// release 构建 MyGo 版的 Windows 便携版，对应 Wails 版的 apps/desktop/scripts/build.bat portable：
// 用 mygo build 编译（exe 带图标、manifest 与版本信息），注入版本号与构建期凭据，再和 lunacli、7z 一起
// 打成 build/release/LunaBox-mygo-<版本>-windows-<架构>-portable.zip（旁边留一份解压好的目录）。
//
//	go run ./tools/release                           # 版本取最近的 v* 标签，架构 amd64
//	go run ./tools/release -version 1.2.0 -arch arm64
//
// 在 apps/desktop-mygo 下运行。凭据与 Wails 版相同：环境变量 LUNABOX_*，没有设置的从 .env.build、.env 读取
// （格式见 .env.build.example）。arm64 与 build.bat 一样动态链接 apps/desktop/lib/winarm64 的 DuckDB。
//
// mygo build 的 -ldflags 是固定的（GOFLAGS 里的 -ldflags 会被它覆盖），所以版本号与凭据不用 -X 注入，
// 而是构建期间在 main 包里写一个 release_gen.go，在 init 里给 lunabox/backend/version 的变量赋值，构建完删除。
// 后端没有在包初始化时读取这些变量，效果与 -X 相同。
package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lunabox/backend/utils"
	"lunabox/backend/version"
)

// genFile 是构建期间写进 main 包的文件（.gitignore 忽略）。
const genFile = "release_gen.go"

// desktopDir 是 Wails 版的目录：lunacli 的源码与随包的 7z、arm64 的 DuckDB 都在这里。
var desktopDir = filepath.Join("..", "desktop")

// credential 是一项构建期凭据：环境变量 env 注入 version 包的变量 name。
type credential struct{ env, name string }

// credentialGroup 同 build.bat 的 configure_ldflags：required 要么都配置、要么都不配置，optional 只能和 required 一起配置。
type credentialGroup struct {
	label    string
	required []credential
	optional []credential
}

var credentialGroups = []credentialGroup{
	{label: "Update Service URL", required: []credential{{"LUNABOX_UPDATE_SERVICE_URL", "UpdateServiceURL"}}},
	{label: "Bangumi OAuth", required: []credential{
		{"LUNABOX_BANGUMI_CLIENT_ID", "BangumiOAuthClientID"},
		{"LUNABOX_BANGUMI_CLIENT_SECRET", "BangumiOAuthClientSecret"},
	}},
	{label: "Hikarinagi OAuth",
		required: []credential{{"LUNABOX_HIKARINAGI_CLIENT_ID", "HikarinagiOAuthClientID"}},
		optional: []credential{{"LUNABOX_HIKARINAGI_CLIENT_SECRET", "HikarinagiOAuthClientSecret"}},
	},
	{label: "Kungal OAuth", required: []credential{{"LUNABOX_KUNGAL_CLIENT_ID", "KungalOAuthClientID"}}},
	{label: "TouchGAL Token", required: []credential{{"LUNABOX_TOUCHGAL_TOKEN", "TouchGalAPIToken"}}},
	{label: "Umbra Registration", required: []credential{
		{"LUNABOX_UMBRA_CLIENT_ID", "UmbraOAuthClientID"},
		{"LUNABOX_UMBRA_REGISTRATION_TOKEN", "UmbraRegistrationToken"},
	}},
}

// assignment 是注入 version 包的一个变量。
type assignment struct{ name, value string }

// toolchain 是 cgo 的编译器与 arm64 动态链接的 DuckDB。
type toolchain struct {
	cc, ccTarget string
	duckdbDLL    string // arm64 时随包的 duckdb.dll，amd64 静态链接为空
}

func main() {
	arch := flag.String("arch", "amd64", "目标架构：amd64 或 arm64")
	ver := flag.String("version", "", "版本号（如 1.2.0）；为空时取最近的 v* 标签")
	flag.Parse()
	// Ctrl+C 交给 mygo build 与 go build，等它们退出后再删除 release_gen.go。
	signal.Ignore(os.Interrupt)
	if err := run(*arch, *ver); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		fmt.Fprintln(os.Stderr, "ERROR: LunaBox MyGo build failed.")
		os.Exit(1)
	}
}

func run(arch, ver string) error {
	switch arch {
	case "x64":
		arch = "amd64"
	case "aarch64":
		arch = "arm64"
	}
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("不支持的架构 %s（amd64 或 arm64）", arch)
	}
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	envFiles := existingFiles(".env.build", ".env")
	if err := utils.LoadEnvFilesIfExists(envFiles...); err != nil {
		return err
	}
	ver, commit, buildTime := resolveVersion(ver)
	creds, status, err := credentials()
	if err != nil {
		return err
	}
	tc, err := configureToolchain(arch)
	if err != nil {
		return err
	}
	sevenZip := filepath.Join(desktopDir, "lib", "win"+arch, "7z")
	for _, name := range []string{"7z.exe", "7z.dll"} {
		if !fileExists(filepath.Join(sevenZip, name)) {
			return fmt.Errorf("缺少 %s", filepath.Join(sevenZip, name))
		}
	}

	fmt.Println("========================================")
	fmt.Println("LunaBox MyGo Windows Portable Build")
	fmt.Println("Target: windows/" + arch)
	fmt.Println("Version: " + ver)
	fmt.Println("Commit: " + commit)
	fmt.Printf("CGO Compiler: %s (%s)\n", tc.cc, tc.ccTarget)
	if len(envFiles) > 0 {
		fmt.Println("Build Env File: " + strings.Join(envFiles, ", "))
	}
	for _, g := range credentialGroups {
		fmt.Printf("%s Injection: %s\n", g.label, status[g.label])
	}
	if tc.duckdbDLL != "" {
		fmt.Println("DuckDB Dynamic DLL: " + tc.duckdbDLL)
	}
	fmt.Println("Bundled 7z: " + sevenZip)
	fmt.Println("========================================")
	fmt.Println()

	vars := append([]assignment{
		{"Version", ver},
		{"GitCommit", commit},
		{"BuildTime", buildTime},
		{"BuildMode", "portable"},
	}, creds...)

	fmt.Println("[portable 1/3] Building GUI (mygo build)...")
	if err := buildApp(arch, vars); err != nil {
		return err
	}

	fmt.Println("[portable 2/3] Building CLI...")
	cli := filepath.Join(cfg.Out, "release", "lunacli-"+arch+".exe")
	if err := buildCLI(arch, cli, vars[:4]); err != nil {
		return err
	}

	fmt.Println("[portable 3/3] Creating ZIP...")
	name := fmt.Sprintf("LunaBox-mygo-%s-windows-%s-portable", ver, arch)
	dir := filepath.Join(cfg.Out, "release", name)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for _, sub := range []string{"backups", "covers", "backgrounds", "logs"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	// mygo build 的输出：主程序改名为 LunaBox.exe（lunacli protocol register 在旁边找它），
	// 其余顶层的 .exe 是 NSIS 安装包（BuildMode 是 portable，不能当安装版用），不放进便携版。
	built := filepath.Join(cfg.Out, "windows-"+arch)
	entries, err := os.ReadDir(built)
	if err != nil {
		return err
	}
	for _, e := range entries {
		src, dst := filepath.Join(built, e.Name()), filepath.Join(dir, e.Name())
		switch {
		case e.Name() == cfg.Name+".exe":
			dst = filepath.Join(dir, "LunaBox.exe")
		case !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".exe"):
			continue
		}
		if err := copyPath(src, dst); err != nil {
			return err
		}
	}
	if !fileExists(filepath.Join(dir, "LunaBox.exe")) {
		return fmt.Errorf("mygo build 没有输出 %s", filepath.Join(built, cfg.Name+".exe"))
	}
	if err := copyPath(cli, filepath.Join(dir, "lunacli.exe")); err != nil {
		return err
	}
	if err := copyPath(sevenZip, filepath.Join(dir, "7z")); err != nil {
		return err
	}
	if tc.duckdbDLL != "" {
		if err := copyPath(tc.duckdbDLL, filepath.Join(dir, "duckdb.dll")); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte(readme(ver)), 0o644); err != nil {
		return err
	}
	zipPath := dir + ".zip"
	if err := writeZip(zipPath, dir); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Build completed successfully.")
	fmt.Println("Portable: " + zipPath)
	fmt.Println("Unpacked: " + dir)
	fmt.Println("========================================")
	return nil
}

// config 是 mygo.json 里用到的字段：mygo build 把 exe 输出到 <out>/windows-<架构>/<name>.exe。
type config struct {
	Name string `json:"name"`
	Out  string `json:"out"`
}

func readConfig() (config, error) {
	data, err := os.ReadFile("mygo.json")
	if err != nil {
		return config{}, fmt.Errorf("读取 mygo.json（要在 apps/desktop-mygo 下运行）：%w", err)
	}
	var c config
	if err := json.Unmarshal(data, &c); err != nil {
		return config{}, fmt.Errorf("mygo.json：%w", err)
	}
	if c.Out == "" {
		c.Out = "build"
	}
	return c, nil
}

// resolveVersion 同 build.bat 的 resolve_version：版本号去掉开头的 v，没有标签时用 version 包的默认值。
func resolveVersion(ver string) (v, commit, buildTime string) {
	if ver == "" {
		ver = gitOutput("describe", "--tags", "--abbrev=0", "--match", "v[0-9]*")
	}
	if ver == "" {
		ver = version.Version
	}
	commit = gitOutput("rev-parse", "--short", "HEAD")
	if commit == "" {
		commit = "unknown"
	}
	return strings.TrimPrefix(ver, "v"), commit, time.Now().Format("2006-01-02 15:04:05")
}

func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// credentials 返回要注入的凭据与每组的状态（enabled、disabled）。
func credentials() ([]assignment, map[string]string, error) {
	var vars []assignment
	status := map[string]string{}
	for _, g := range credentialGroups {
		var set, missing []string
		for _, c := range g.required {
			if v := strings.TrimSpace(os.Getenv(c.env)); v != "" {
				set = append(set, c.env)
				vars = append(vars, assignment{c.name, v})
			} else {
				missing = append(missing, c.env)
			}
		}
		if len(set) > 0 && len(missing) > 0 {
			return nil, nil, fmt.Errorf("%s 必须同时配置（缺少 %s）", strings.Join(append(set, missing...), " 与 "), strings.Join(missing, "、"))
		}
		for _, c := range g.optional {
			if v := strings.TrimSpace(os.Getenv(c.env)); v != "" {
				if len(set) == 0 {
					return nil, nil, fmt.Errorf("%s 需要同时配置 %s", c.env, g.required[0].env)
				}
				vars = append(vars, assignment{c.name, v})
			}
		}
		status[g.label] = "disabled"
		if len(set) > 0 {
			status[g.label] = "enabled"
		}
	}
	return vars, status, nil
}

// configureToolchain 同 build.bat 的 configure_architecture：设置 cgo 的环境变量（mygo build 与 go build 继承），
// arm64 用 MSYS2 的 clang 并动态链接 apps/desktop/lib/winarm64 的 DuckDB（duckdb_use_lib）。
func configureToolchain(arch string) (toolchain, error) {
	var tc toolchain
	os.Setenv("CGO_ENABLED", "1")
	if arch == "arm64" {
		src := filepath.Join(desktopDir, "lib", "winarm64")
		dst, err := filepath.Abs(filepath.Join("build", "duckdb", "winarm64"))
		if err != nil {
			return tc, err
		}
		for _, name := range []string{"duckdb.dll", "duckdb.lib"} {
			if !fileExists(filepath.Join(src, name)) {
				return tc, fmt.Errorf("缺少 %s", filepath.Join(src, name))
			}
		}
		if err := copyPath(filepath.Join(src, "duckdb.dll"), filepath.Join(dst, "duckdb.dll")); err != nil {
			return tc, err
		}
		if err := copyPath(filepath.Join(src, "duckdb.lib"), filepath.Join(dst, "libduckdb.dll.a")); err != nil {
			return tc, err
		}
		const triple = "aarch64-w64-windows-gnu"
		var bin string
		for _, root := range []string{os.Getenv("MSYS2_LOCATION"), `C:\msys64`} {
			if root != "" && fileExists(filepath.Join(root, "clangarm64", "bin", "clang.exe")) {
				bin = filepath.Join(root, "clangarm64", "bin")
				break
			}
		}
		if bin != "" {
			os.Setenv("CC", filepath.Join(bin, "clang.exe")+" --target="+triple)
			if fileExists(filepath.Join(bin, "clang++.exe")) {
				os.Setenv("CXX", filepath.Join(bin, "clang++.exe")+" --target="+triple)
			}
			prependPath(bin)
		} else if os.Getenv("CC") == "" {
			if _, err := exec.LookPath("aarch64-w64-mingw32-gcc"); err == nil {
				os.Setenv("CC", "aarch64-w64-mingw32-gcc")
			}
		}
		os.Setenv("CGO_LDFLAGS", "-L"+dst+" -lduckdb")
		// mygo build 把 GOFLAGS 里的 -tags 和它自己的合在一起。
		os.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -tags=duckdb_use_lib"))
		prependPath(dst)
		tc.duckdbDLL = filepath.Join(dst, "duckdb.dll")
	}
	if os.Getenv("CC") == "" {
		if _, err := exec.LookPath("gcc"); err == nil {
			os.Setenv("CC", "gcc")
		}
	}
	tc.cc = os.Getenv("CC")
	if tc.cc == "" {
		return tc, fmt.Errorf("windows/%s 的 cgo 构建需要对应架构的 C 编译器：安装 MSYS2 与它的 %s 工具链，或设置 CC", arch, arch)
	}
	fields := strings.Fields(tc.cc)
	out, err := exec.Command(fields[0], append(fields[1:], "-dumpmachine")...).Output()
	if err != nil {
		return tc, fmt.Errorf("无法检查 C 编译器的目标平台：%s：%w", tc.cc, err)
	}
	tc.ccTarget = strings.TrimSpace(string(out))
	want := []string{"x86_64", "amd64"}
	if arch == "arm64" {
		want = []string{"aarch64", "arm64"}
	}
	if !strings.Contains(tc.ccTarget, want[0]) && !strings.Contains(tc.ccTarget, want[1]) {
		return tc, fmt.Errorf("C 编译器的目标平台 %s 与 %s 不符（CC=%s）", tc.ccTarget, arch, tc.cc)
	}
	return tc, nil
}

func prependPath(dir string) {
	os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// buildApp 写出 release_gen.go 后运行 mygo build，构建完（包括失败、中断）删除它。
func buildApp(arch string, vars []assignment) error {
	var b strings.Builder
	b.WriteString("// Code generated by tools/release; DO NOT EDIT.\n\npackage main\n\nimport \"lunabox/backend/version\"\n\nfunc init() {\n")
	for _, v := range vars {
		fmt.Fprintf(&b, "\tversion.%s = %s\n", v.name, strconv.Quote(v.value))
	}
	b.WriteString("}\n")
	if err := os.WriteFile(genFile, []byte(b.String()), 0o644); err != nil {
		return err
	}
	defer os.Remove(genFile)
	cmd := exec.Command("go", "tool", "mygo", "build", "-platform", "windows/"+arch)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mygo build：%w", err)
	}
	return nil
}

// buildCLI 同 build.bat 的 build_cli：用 Wails 版模块里的 cmd/lunacli，注入版本号与 BuildMode。
func buildCLI(arch, out string, vars []assignment) error {
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	ldflags := "-s -w"
	for _, v := range vars {
		ldflags += " -X " + ldflagsQuote("lunabox/backend/version."+v.name+"="+v.value)
	}
	cmd := exec.Command("go", "-C", desktopDir, "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", abs, "./cmd/lunacli")
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH="+arch)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("构建 lunacli：%w", err)
	}
	return nil
}

// ldflagsQuote 给 -ldflags 里含空格的参数加引号（go build 按空格拆分 -ldflags，引号里不转义）。
func ldflagsQuote(s string) string {
	if !strings.ContainsAny(s, " \t'\"") {
		return s
	}
	if strings.Contains(s, "'") {
		return `"` + s + `"`
	}
	return "'" + s + "'"
}

func readme(ver string) string {
	return strings.Join([]string{
		"LunaBox (MyGo) Portable v" + ver,
		"",
		"This package contains:",
		"  - LunaBox.exe  : GUI version (double-click to launch)",
		"  - lunacli.exe  : CLI version (use in a terminal)",
		"",
		"Data (database, settings, covers, logs) is stored next to LunaBox.exe.",
		"",
		"CLI usage:",
		"  lunacli list",
		"  lunacli start <game-id>",
		"  lunacli protocol register",
		"  lunacli protocol unregister",
		"  lunacli help",
		"",
	}, "\r\n")
}

// writeZip 把 dir 打成 zip，里面是以 dir 的名字命名的顶层目录（同 build.bat 的 Compress-Archive）。
func writeZip(path, dir string) (err error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	zw := zip.NewWriter(f)
	parent := filepath.Dir(dir)
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(parent, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			h.Name += "/"
			_, err = zw.CreateHeader(h)
			return err
		}
		h.Method = zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		src, err := os.Open(p)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
	if err != nil {
		return err
	}
	return zw.Close()
}

// copyPath 复制文件或目录（目录递归复制），需要时创建目标所在的目录。
func copyPath(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func existingFiles(names ...string) []string {
	var out []string
	for _, n := range names {
		if fileExists(n) {
			out = append(out, n)
		}
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
