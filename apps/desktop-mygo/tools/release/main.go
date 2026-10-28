// release 构建 MyGo 版的发布包：用 mygo build 编译（Windows 的 exe 带图标、manifest 与版本信息），
// 注入版本号与构建期凭据，再和 lunacli、7z 一起打包到 build/release。
// Windows 是便携版 LunaBox-mygo-<版本>-windows-<架构>-portable.zip（旁边留一份解压好的目录），
// 对应 Wails 版的 apps/desktop/scripts/build.bat portable。
// Linux 是 LunaBox-mygo-<版本>-linux-<架构>.AppImage（旁边留 .AppDir），对应 build.sh appimage，
// BuildMode 是 appimage：数据在 ~/.config/LunaBox，与 Wails 版的 AppImage 相同；需要 appimagetool。
//
//	go run ./tools/release                           # 版本取最近的 v* 标签，架构 amd64
//	go run ./tools/release -version 1.2.0 -arch arm64
//
// 在 apps/desktop-mygo 下运行，目标系统是当前系统（Windows 或 Linux）：DuckDB 需要 cgo，不能交叉编译。
// 凭据与 Wails 版相同：环境变量 LUNABOX_*，没有设置的从 .env.build、.env 读取（格式见 .env.build.example）。
// Windows arm64 与 build.bat 一样动态链接 apps/desktop/lib/winarm64 的 DuckDB。
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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"lunabox/backend/utils"
)

// genFile 是构建期间写进 main 包的文件（.gitignore 忽略）。
const genFile = "release_gen.go"

// desktopDir 是 Wails 版的目录：lunacli 的源码与随包的 7z、Windows arm64 的 DuckDB 都在这里。
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

// toolchain 是 cgo 的编译器与 Windows arm64 动态链接的 DuckDB。
type toolchain struct {
	cc, ccTarget string
	duckdbDLL    string // Windows arm64 时随包的 duckdb.dll，其余静态链接为空
}

// target 是一个目标平台的构建差异。
type target struct {
	goos, arch string
	label      string // 输出里的包名
	exeSuffix  string
	sevenZip   string // 随包的 7z：Windows 是目录（7z.exe、7z.dll），Linux 是 7zz
	buildMode  string // version.BuildMode
}

func newTarget(goos, arch string) target {
	if goos == "linux" {
		return target{goos, arch, "Linux AppImage", "", filepath.Join(desktopDir, "lib", "linux"+arch, "7z", "7zz"), "appimage"}
	}
	return target{goos, arch, "Windows Portable", ".exe", filepath.Join(desktopDir, "lib", "win"+arch, "7z"), "portable"}
}

// sevenZipFiles 是随包的 7z 必须有的文件。
func (t target) sevenZipFiles() []string {
	if t.goos == "linux" {
		return []string{t.sevenZip}
	}
	return []string{filepath.Join(t.sevenZip, "7z.exe"), filepath.Join(t.sevenZip, "7z.dll")}
}

// sevenZipDest 是 7z 在主程序所在目录 dir 里的位置，即后端 archiveutils 找它的地方。
func (t target) sevenZipDest(dir string) string {
	if t.goos == "linux" {
		return filepath.Join(dir, "7zz")
	}
	return filepath.Join(dir, "7z")
}

// builtExe 是 mygo build 输出的主程序名：Windows 是 <name>.exe，Linux 是 slug 化的 name。
func (t target) builtExe(name string) string {
	if t.goos == "linux" {
		return slugify(name)
	}
	return name + ".exe"
}

// skipBuilt 报告 mygo build 的输出里不放进发布包的文件：Windows 的 NSIS 安装包（BuildMode 是 portable，
// 不能当安装版用），Linux 的 deb、tar.gz、install.sh 与桌面入口（AppImage 有自己的桌面入口）。
func (t target) skipBuilt(e fs.DirEntry) bool {
	if e.IsDir() {
		return false
	}
	name := strings.ToLower(e.Name())
	if t.goos == "linux" {
		return name == "install.sh" || strings.HasSuffix(name, ".deb") || strings.HasSuffix(name, ".tar.gz") ||
			strings.HasSuffix(name, ".desktop") || strings.HasSuffix(name, ".xml")
	}
	return filepath.Ext(name) == ".exe"
}

// slugRe 与 slugify 同 mygo 的 cmd/mygo/init.go：Linux 的主程序名。
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if s == "" {
		return "app"
	}
	return s
}

func main() {
	arch := flag.String("arch", "amd64", "目标架构：amd64 或 arm64")
	ver := flag.String("version", "", "版本号（如 1.2.0）；为空时取最近的 v* 标签")
	skipAppImage := flag.Bool("skip-appimage", false, "Linux：只打 RPM/deb，不生成 AppImage（不需要 appimagetool，可离线）")
	flag.Parse()
	// Ctrl+C 交给 mygo build 与 go build，等它们退出后再删除 release_gen.go。
	signal.Ignore(os.Interrupt)
	if err := run(*arch, *ver, *skipAppImage); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		fmt.Fprintln(os.Stderr, "ERROR: LunaBox MyGo build failed.")
		os.Exit(1)
	}
}

func run(arch, ver string, skipAppImage bool) error {
	switch arch {
	case "x64":
		arch = "amd64"
	case "aarch64":
		arch = "arm64"
	}
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("不支持的架构 %s（amd64 或 arm64）", arch)
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return fmt.Errorf("不支持在 %s 上构建（Windows 或 Linux）", runtime.GOOS)
	}
	t := newTarget(runtime.GOOS, arch)
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	envFiles := existingFiles(".env.build", ".env")
	if err := utils.LoadEnvFilesIfExists(envFiles...); err != nil {
		return err
	}
	ver, commit, buildTime := resolveVersion(ver, cfg.Version)
	// mygo build 用 mygo.json 的 version 写 exe 的文件版本（同 Wails 版的 build/windows/info.json），
	// 应用里显示的版本是 ver；两者不一致说明 mygo.json 没有随 apps/desktop/scripts/update-build-assets 更新。
	if cfg.Version != baseVersion(ver) {
		fmt.Fprintf(os.Stderr, "WARNING: mygo.json 的 version %s 与版本号 %s 不一致，exe 的文件版本会是 %s；运行 apps/desktop/scripts/update-build-assets 同步。\n", cfg.Version, ver, cfg.Version)
	}
	creds, status, err := credentials()
	if err != nil {
		return err
	}
	tc, err := configureToolchain(t.goos, arch)
	if err != nil {
		return err
	}
	for _, p := range t.sevenZipFiles() {
		if !fileExists(p) {
			return fmt.Errorf("缺少 %s", p)
		}
	}
	var appImageTool string
	if t.goos == "linux" && !skipAppImage {
		if appImageTool, err = exec.LookPath("appimagetool"); err != nil {
			return fmt.Errorf("打包 AppImage 需要 appimagetool（https://github.com/AppImage/appimagetool）：%w", err)
		}
	}

	fmt.Println("========================================")
	fmt.Println("LunaBox MyGo " + t.label + " Build")
	fmt.Println("Target: " + t.goos + "/" + arch)
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
	fmt.Println("Bundled 7z: " + t.sevenZip)
	fmt.Println("========================================")
	fmt.Println()

	vars := append([]assignment{
		{"Version", ver},
		{"GitCommit", commit},
		{"BuildTime", buildTime},
		{"BuildMode", t.buildMode},
	}, creds...)

	steps := 3
	if t.goos == "linux" {
		steps = 4 // 多一步 RPM/deb
		if skipAppImage {
			steps = 3
		}
	}

	fmt.Printf("[1/%d] Building GUI (mygo build)...\n", steps)
	if err := buildApp(t, vars); err != nil {
		return err
	}

	fmt.Printf("[2/%d] Building CLI...\n", steps)
	cli := filepath.Join(cfg.Out, "release", "lunacli-"+t.goos+"-"+arch+t.exeSuffix)
	if err := buildCLI(t, cli, vars[:4]); err != nil {
		return err
	}

	release := filepath.Join(cfg.Out, "release")
	var pkg, dir string
	if t.goos == "linux" {
		if !skipAppImage {
			fmt.Printf("[3/%d] Creating AppImage...\n", steps)
			name := fmt.Sprintf("LunaBox-mygo-%s-linux-%s", ver, arch)
			pkg, dir = filepath.Join(release, name+".AppImage"), filepath.Join(release, name+".AppDir")
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			if err := stageFiles(cfg, t, tc, cli, filepath.Join(dir, "usr", "bin")); err != nil {
				return err
			}
			if err := writeAppImage(appImageTool, t, ver, dir, pkg); err != nil {
				return err
			}
		}
		fmt.Printf("[%d/%d] Creating RPM and deb...\n", steps, steps)
		if err := buildLinuxPackages(cfg, t, arch, ver, vars); err != nil {
			return err
		}
		pkg = filepath.Join(release, fmt.Sprintf("LunaBox-mygo-%s-linux-%s.rpm", ver, arch))
	} else {
		fmt.Printf("[3/%d] Creating ZIP...\n", steps)
		name := fmt.Sprintf("LunaBox-mygo-%s-windows-%s-portable", ver, arch)
		pkg, dir = filepath.Join(release, name+".zip"), filepath.Join(release, name)
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		for _, sub := range []string{"backups", "covers", "backgrounds", "logs"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
				return err
			}
		}
		if err := stageFiles(cfg, t, tc, cli, dir); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte(readme(ver)), 0o644); err != nil {
			return err
		}
		if err := writeZip(pkg, dir); err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Build completed successfully.")
	fmt.Println("Package: " + pkg)
	fmt.Println("Unpacked: " + dir)
	fmt.Println("========================================")
	return nil
}

// stageFiles 把主程序、lunacli、7z（与 Windows arm64 的 duckdb.dll）放进 dir。
// mygo build 的输出里主程序改名为 LunaBox(.exe)（lunacli protocol register 在旁边找它），
// 其余文件（资源）原样放在旁边，安装包之类见 skipBuilt。
func stageFiles(cfg config, t target, tc toolchain, cli, dir string) error {
	built := filepath.Join(cfg.Out, t.goos+"-"+t.arch)
	entries, err := os.ReadDir(built)
	if err != nil {
		return err
	}
	builtExe := t.builtExe(cfg.Name)
	gui := filepath.Join(dir, "LunaBox"+t.exeSuffix)
	for _, e := range entries {
		src, dst := filepath.Join(built, e.Name()), filepath.Join(dir, e.Name())
		switch {
		case e.Name() == builtExe:
			dst = gui
		case t.skipBuilt(e):
			continue
		}
		if err := copyPath(src, dst); err != nil {
			return err
		}
	}
	if !fileExists(gui) {
		return fmt.Errorf("mygo build 没有输出 %s", filepath.Join(built, builtExe))
	}
	if err := copyPath(cli, filepath.Join(dir, "lunacli"+t.exeSuffix)); err != nil {
		return err
	}
	if err := copyPath(t.sevenZip, t.sevenZipDest(dir)); err != nil {
		return err
	}
	if t.goos == "linux" {
		for _, p := range []string{gui, filepath.Join(dir, "lunacli"), t.sevenZipDest(dir)} {
			if err := os.Chmod(p, 0o755); err != nil {
				return err
			}
		}
	}
	if tc.duckdbDLL != "" {
		if err := copyPath(tc.duckdbDLL, filepath.Join(dir, "duckdb.dll")); err != nil {
			return err
		}
	}
	return nil
}

// appID 是 Linux 桌面入口与图标的名字，同 Wails 版 build.sh 的 AppImage 与后端 protocol 包的 linuxDesktopID。
const appID = "io.github.saramanda9988.lunabox"

// writeAppImage 同 build.sh 的 AppImage 打包：在 AppDir（主程序等已在 usr/bin）里写 AppRun、桌面入口与图标，
// 再用 appimagetool 打成 pkg。AppRun 不设置 Wails 版给 WebKitGTK 的环境变量（MyGo 不用 WebView）。
func writeAppImage(tool string, t target, ver, dir, pkg string) error {
	if err := os.WriteFile(filepath.Join(dir, "AppRun"), []byte(appRun), 0o755); err != nil {
		return err
	}
	desktop := []byte(appImageDesktopEntry(ver))
	for _, p := range []string{
		filepath.Join(dir, appID+".desktop"),
		filepath.Join(dir, "usr", "share", "applications", appID+".desktop"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, desktop, 0o644); err != nil {
			return err
		}
	}
	icon := filepath.Join("resources", "icon.png")
	for _, p := range []string{
		filepath.Join(dir, appID+".png"),
		filepath.Join(dir, "usr", "share", "icons", "hicolor", "512x512", "apps", appID+".png"),
	} {
		if err := copyPath(icon, p); err != nil {
			return err
		}
	}
	if err := os.Remove(pkg); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	appImageArch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[t.arch]
	cmd := exec.Command(tool, dir, pkg)
	cmd.Env = append(os.Environ(), "ARCH="+appImageArch)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("appimagetool：%w", err)
	}
	return os.Chmod(pkg, 0o755)
}

// appRun 同 build.sh 的 write_linux_appimage_apprun：告诉后端 AppImage 的路径（协议注册、CLI 包装脚本用），
// 第一个参数是 cli 或 lunacli 时运行 lunacli。
const appRun = `#!/usr/bin/env sh
set -eu
APP_RUN_PATH="$(readlink -f "$0" 2>/dev/null || printf '%s' "$0")"
APP_DIR="$(CDPATH= cd "$(dirname "$APP_RUN_PATH")" && pwd -P)"
if [ -n "${APPIMAGE:-}" ]; then
    export LUNABOX_APPIMAGE_PATH="$APPIMAGE"
fi
case "${1:-}" in
    cli|lunacli)
        shift
        exec "$APP_DIR/usr/bin/lunacli" "$@"
        ;;
esac
exec "$APP_DIR/usr/bin/LunaBox" "$@"
`

// appImageDesktopEntry 同 build.sh 的 write_linux_appimage_desktop。MyGo 不设置 GTK 的程序名，
// 窗口的 WM_CLASS 取自可执行文件名 LunaBox。
func appImageDesktopEntry(ver string) string {
	return `[Desktop Entry]
Version=1.0
Type=Application
Name=LunaBox
Comment=LunaBox game library manager
Exec=LunaBox %u
Terminal=false
Icon=` + appID + `
Categories=Game;
StartupWMClass=LunaBox
MimeType=x-scheme-handler/lunabox;
X-AppImage-Name=LunaBox
X-AppImage-Version=` + ver + `
`
}

// config 是 mygo.json 里用到的字段：mygo build 把 exe 输出到 <out>/windows-<架构>/<name>.exe。
type config struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Out     string `json:"out"`
}

// baseVersion 是版本号的 X.Y.Z 部分（去掉 -dev.N+commit 之类的后缀）。
func baseVersion(v string) string {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		return v[:i]
	}
	return v
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

// resolveVersion 同 build.bat 的 resolve_version：版本号去掉开头的 v，没有标签时用 1.0.0。
// resolveVersion 同 build.bat 的 resolve_version：版本号去掉开头的 v。
// 拿不到 git 标签时依次退回 mygo.json 的版本与 1.0.0（容器里挂载 worktree、没装 git 时会走到这里）。
func resolveVersion(ver, fallback string) (v, commit, buildTime string) {
	if ver == "" {
		ver = gitOutput("describe", "--tags", "--abbrev=0", "--match", "v[0-9]*")
	}
	if ver == "" {
		ver = fallback
	}
	if ver == "" {
		ver = "1.0.0"
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
// Windows arm64 用 MSYS2 的 clang 并动态链接 apps/desktop/lib/winarm64 的 DuckDB（duckdb_use_lib）；
// Linux 与 build.sh 相同，用系统的 gcc 静态链接 DuckDB。
func configureToolchain(goos, arch string) (toolchain, error) {
	var tc toolchain
	os.Setenv("CGO_ENABLED", "1")
	if goos == "windows" && arch == "arm64" {
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
		if goos == "linux" {
			return tc, fmt.Errorf("linux/%s 的 cgo 构建需要 C 编译器：安装 gcc（如 build-essential），或设置 CC", arch)
		}
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
func buildApp(t target, vars []assignment) error {
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
	cmd := exec.Command("go", "tool", "mygo", "build", "-platform", t.goos+"/"+t.arch)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mygo build：%w", err)
	}
	return nil
}

// buildCLI 同 build.bat 的 build_cli：用 Wails 版模块里的 cmd/lunacli，注入版本号与 BuildMode。
func buildCLI(t target, out string, vars []assignment) error {
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	ldflags := "-s -w"
	for _, v := range vars {
		ldflags += " -X " + ldflagsQuote("lunabox/backend/version."+v.name+"="+v.value)
	}
	cmd := exec.Command("go", "-C", desktopDir, "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", abs, "./cmd/lunacli")
	cmd.Env = append(os.Environ(), "GOOS="+t.goos, "GOARCH="+t.arch)
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

// buildLinuxPackages 用 nfpm 打 RPM 与 deb（仓库的 Wails 版就是用它打这两个包，见 scripts/build.sh）。
//
// 安装包用 BuildMode=installer 再编译一次：数据同样在 ~/.config/LunaBox，但取可执行文件路径时不看
// APPIMAGE 环境变量，开机自启与 lunabox:// 协议才会指向 /usr/bin/lunabox 而不是临时挂载点。
func buildLinuxPackages(cfg config, t target, arch, ver string, vars []assignment) error {
	nfpm, err := exec.LookPath("nfpm")
	if err != nil {
		return fmt.Errorf("打 RPM/deb 需要 nfpm：go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest")
	}
	installed := make([]assignment, len(vars))
	for i, v := range vars {
		if v.name == "BuildMode" {
			v.value = "installer"
		}
		installed[i] = v
	}
	if err := buildApp(t, installed); err != nil {
		return err
	}
	release := filepath.Join(cfg.Out, "release")
	cli := filepath.Join(release, "lunacli-linux-"+arch+"-installer")
	if err := buildCLI(t, cli, installed[:4]); err != nil {
		return err
	}
	built := filepath.Join(cfg.Out, t.goos+"-"+t.arch)
	app := filepath.Join(built, t.builtExe(cfg.Name))
	if !fileExists(app) {
		return fmt.Errorf("mygo build 没有输出 %s", app)
	}
	desktop, err := firstFile(built, ".desktop")
	if err != nil {
		return fmt.Errorf("mygo build 没有输出桌面项：%w", err)
	}
	icon, err := firstFile(built, ".png")
	if err != nil {
		return fmt.Errorf("mygo build 没有输出图标：%w", err)
	}
	abs := func(path string) string {
		p, err := filepath.Abs(path)
		if err != nil {
			return path
		}
		return p
	}
	version, release_ := splitRPMVersion(ver)
	scripts := filepath.Join(desktopDir, "build", "linux", "nfpm", "scripts")
	cfgPath := filepath.Join(release, "nfpm-linux-"+arch+".yaml")
	body := fmt.Sprintf(`name: "lunabox"
arch: %q
platform: "linux"
version: %q
release: %q
section: "utils"
priority: "optional"
maintainer: "LunaBox"
description: "LunaBox game library manager (MyGo build)"
vendor: "LunaBox"
homepage: "https://github.com/Saramanda9988/LunaBox"
license: "MIT"

contents:
  - src: %q
    dst: "/usr/bin/lunabox"
    file_info:
      mode: 0755
  - src: %q
    dst: "/usr/bin/lunacli"
    file_info:
      mode: 0755
  - src: %q
    dst: "/usr/lib/lunabox/7zz"
    file_info:
      mode: 0755
  - src: %q
    dst: "/usr/share/applications/lunabox.desktop"
  - src: %q
    dst: "/usr/share/icons/hicolor/512x512/apps/lunabox.png"

depends:
  - "libgtk-3-0 | libgtk-3-0t64"
  - "libayatana-appindicator3-1"

overrides:
  rpm:
    depends:
      - gtk3
      - libayatana-appindicator-gtk3

scripts:
  postinstall: %q
  postremove: %q
`, arch, version, release_, abs(app), abs(cli), abs(t.sevenZip), abs(desktop), abs(icon),
		abs(filepath.Join(scripts, "postinstall.sh")), abs(filepath.Join(scripts, "postremove.sh")))
	if err := os.MkdirAll(release, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		return err
	}
	for _, packager := range []string{"rpm", "deb"} {
		out := filepath.Join(release, fmt.Sprintf("LunaBox-mygo-%s-linux-%s.%s", ver, arch, packager))
		cmd := exec.Command(nfpm, "pkg", "--config", cfgPath, "--packager", packager, "--target", out)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("nfpm 打 %s：%w", packager, err)
		}
	}
	return nil
}

// splitRPMVersion 把 "1.2.3-beta" 拆成 RPM 的 version 与 release：RPM 的 version 里不能有“-”。
func splitRPMVersion(ver string) (version, release string) {
	version, release, found := strings.Cut(ver, "-")
	if !found || release == "" {
		return version, "1"
	}
	return version, release
}

// firstFile 返回 dir 下第一个以 suffix 结尾的文件。
func firstFile(dir, suffix string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("%s 下没有 %s 文件", dir, suffix)
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

// copyFile 复制一个文件，保留它的权限（Linux 的可执行位）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
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
