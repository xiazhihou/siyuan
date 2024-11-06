// SiYuan - Refactor your thinking
// Copyright (c) 2020-present, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package util

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/88250/gulu"
	figure "github.com/common-nighthawk/go-figure"
	"github.com/gofrs/flock"
	"github.com/siyuan-note/filelock"
	"github.com/siyuan-note/httpclient"
	"github.com/siyuan-note/logging"
)

// var Mode = "dev"
var Mode = "prod"

const (
	Ver       = "3.1.11"
	IsInsider = false
)

var (
	RunInContainer             = false // 是否运行在容器中
	SiyuanAccessAuthCodeBypass = true  // 是否跳过空访问授权码检查
)

// initEnvVars 初始化环境变量，检查是否在 Docker 容器中运行，并解析 SIYUAN_ACCESS_AUTH_CODE_BYPASS 环境变量。
// 如果解析环境变量失败，则默认 SiyuanAccessAuthCodeBypass 为 false。
func initEnvVars() {
	RunInContainer = isRunningInDockerContainer()
	var err error
	if SiyuanAccessAuthCodeBypass, err = strconv.ParseBool(os.Getenv("SIYUAN_ACCESS_AUTH_CODE_BYPASS")); err != nil {
		SiyuanAccessAuthCodeBypass = true
	}
}

var (
	bootProgress = atomic.Int32{} // 启动进度，从 0 到 100
	bootDetails  string           // 启动细节描述
	HttpServing  = false          // 是否 HTTP 伺服已经可用
)

// Boot 函数负责启动 SiYuan 内核的初始化过程。
// 它包括初始化环境变量、增加启动进度、设置随机种子、初始化 MIME 类型、HTTP 客户端等。
// 通过命令行参数配置工作空间路径、工作目录、HTTP 服务器端口、只读模式、访问授权码、SSL 设置、语言和运行模式。
// 如果在 Docker 容器中运行且未提供访问授权码，则会中断启动过程。
// 初始化工作空间目录，并根据运行模式设置主题和图标路径。
// 最后，显示启动横幅并记录启动信息。
func Boot() {
	initEnvVars()
	IncBootProgress(3, "Booting kernel...")
	rand.Seed(time.Now().UTC().UnixNano())
	initMime()
	initHttpClient()

	// workspacePath 指定了工作空间的目录路径。如果不提供，则默认为用户的 ~/SiYuan/ 目录。
	// 该参数用于指定 SiYuan 系统的工作空间位置，以便系统能够正确地读取和写入相关文件。
	workspacePath := flag.String("workspace", "", "dir path of the workspace, default to ~/SiYuan/")
	wdPath := flag.String("wd", WorkingDir, "working directory of SiYuan")
	port := flag.String("port", "0", "port of the HTTP server")
	readOnly := flag.String("readonly", "false", "read-only mode")
	accessAuthCode := flag.String("accessAuthCode", "", "access auth code")
	ssl := flag.Bool("ssl", false, "for https and wss")
	lang := flag.String("lang", "zh_CN", "zh_CN/zh_CHT/en_US/fr_FR/es_ES/ja_JP/it_IT/de_DE/he_IL/ru_RU/pl_PL")
	mode := flag.String("mode", "prod", "dev/prod")
	flag.Parse()

	if "" != *wdPath {
		WorkingDir = *wdPath
	}
	if "" != *lang {
		Lang = *lang
	}
	Mode = *mode
	ServerPort = *port
	ReadOnly, _ = strconv.ParseBool(*readOnly)
	AccessAuthCode = *accessAuthCode
	Container = ContainerStd
	if RunInContainer {
		Container = ContainerDocker
		logging.LogInfof("启动初始化，读取配置AccessAuthCode： %s", AccessAuthCode)
		if "" == AccessAuthCode {
			interruptBoot := true

			// Set the env `SIYUAN_ACCESS_AUTH_CODE_BYPASS=true` to skip checking empty access auth code https://github.com/siyuan-note/siyuan/issues/9709
			if SiyuanAccessAuthCodeBypass {
				interruptBoot = false
				fmt.Println("bypass access auth code check since the env [SIYUAN_ACCESS_AUTH_CODE_BYPASS] is set to [true]")
			}

			if interruptBoot {
				// The access authorization code command line parameter must be set when deploying via Docker https://github.com/siyuan-note/siyuan/issues/9328
				fmt.Printf("the access authorization code command line parameter (--accessAuthCode) must be set when deploying via Docker")
				os.Exit(1)
			}
		}
	}
	if ContainerStd != Container {
		ServerPort = FixedPort
	}

	msStoreFilePath := filepath.Join(WorkingDir, "ms-store")
	ISMicrosoftStore = gulu.File.IsExist(msStoreFilePath)

	UserAgent = UserAgent + " " + Container + "/" + runtime.GOOS
	httpclient.SetUserAgent(UserAgent)

	initWorkspaceDir(*workspacePath)

	SSL = *ssl
	LogPath = filepath.Join(TempDir, "siyuan.log")
	logging.SetLogPath(LogPath)

	// 工作空间仅允许被一个内核进程伺服
	tryLockWorkspace()

	AppearancePath = filepath.Join(ConfDir, "appearance")
	if "dev" == Mode {
		ThemesPath = filepath.Join(WorkingDir, "appearance", "themes")
		IconsPath = filepath.Join(WorkingDir, "appearance", "icons")
	} else {
		ThemesPath = filepath.Join(AppearancePath, "themes")
		IconsPath = filepath.Join(AppearancePath, "icons")
	}

	initPathDir()

	bootBanner := figure.NewColorFigure("SiYuan", "isometric3", "green", true)
	logging.LogInfof("\n" + bootBanner.String())
	logBootInfo()
}

var bootDetailsLock = sync.Mutex{}

func setBootDetails(details string) {
	bootDetailsLock.Lock()
	bootDetails = "v" + Ver + " " + details
	bootDetailsLock.Unlock()
}

func SetBootDetails(details string) {
	if 100 <= bootProgress.Load() {
		return
	}
	setBootDetails(details)
}

func IncBootProgress(progress int32, details string) {
	if 100 <= bootProgress.Load() {
		return
	}
	bootProgress.Add(progress)
	setBootDetails(details)
}

func IsBooted() bool {
	return 100 <= bootProgress.Load()
}

func GetBootProgressDetails() (progress int32, details string) {
	progress = bootProgress.Load()
	bootDetailsLock.Lock()
	details = bootDetails
	bootDetailsLock.Unlock()
	return
}

func GetBootProgress() int32 {
	return bootProgress.Load()
}

func SetBooted() {
	setBootDetails("Finishing boot...")
	bootProgress.Store(100)
	logging.LogInfof("kernel booted")
}

var (
	HomeDir, _    = gulu.OS.Home()
	WorkingDir, _ = os.Getwd()

	WorkspaceDir       string        // 工作空间目录路径
	WorkspaceName      string        // 工作空间名称
	WorkspaceLock      *flock.Flock  // 工作空间锁
	ConfDir            string        // 配置目录路径
	DataDir            string        // 数据目录路径
	RepoDir            string        // 仓库目录路径
	HistoryDir         string        // 数据历史目录路径
	TempDir            string        // 临时目录路径
	LogPath            string        // 配置目录下的日志文件 siyuan.log 路径
	DBName             = "siyuan.db" // SQLite 数据库文件名
	DBPath             string        // SQLite 数据库文件路径
	HistoryDBPath      string        // SQLite 历史数据库文件路径
	AssetContentDBPath string        // SQLite 资源文件内容数据库文件路径
	BlockTreeDBPath    string        // 区块树数据库文件路径
	AppearancePath     string        // 配置目录下的外观目录 appearance/ 路径
	ThemesPath         string        // 配置目录下的外观目录下的 themes/ 路径
	IconsPath          string        // 配置目录下的外观目录下的 icons/ 路径
	SnippetsPath       string        // 数据目录下的 snippets/ 路径

	UIProcessIDs = sync.Map{} // UI 进程 ID
)

// initWorkspaceDir 初始化工作空间目录。
// 该函数首先检查配置文件是否存在，如果不存在则创建配置文件夹。
// 然后根据配置文件或默认路径设置工作空间目录。
// 如果指定了工作空间参数，则使用该参数作为工作空间目录。
// 如果指定的工作空间目录不存在，则使用默认工作空间目录，并创建该目录。
// 最后，设置各种与工作空间相关的路径和环境变量。
func initWorkspaceDir(workspaceArg string) {
	userHomeConfDir := filepath.Join(HomeDir, ".config", "siyuan")
	workspaceConf := filepath.Join(userHomeConfDir, "workspace.json")
	logging.SetLogPath(filepath.Join(userHomeConfDir, "kernel.log"))

	if !gulu.File.IsExist(workspaceConf) {
		if err := os.MkdirAll(userHomeConfDir, 0755); err != nil && !os.IsExist(err) {
			logging.LogErrorf("create user home conf folder [%s] failed: %s", userHomeConfDir, err)
			os.Exit(logging.ExitCodeInitWorkspaceErr)
		}
	}

	defaultWorkspaceDir := filepath.Join(HomeDir, "SiYuan")
	if gulu.OS.IsWindows() {
		// 改进 Windows 端默认工作空间路径 https://github.com/siyuan-note/siyuan/issues/5622
		if userProfile := os.Getenv("USERPROFILE"); "" != userProfile {
			defaultWorkspaceDir = filepath.Join(userProfile, "SiYuan")
		}
	}

	var workspacePaths []string
	if !gulu.File.IsExist(workspaceConf) {
		WorkspaceDir = defaultWorkspaceDir
	} else {
		workspacePaths, _ = ReadWorkspacePaths()
		if 0 < len(workspacePaths) {
			WorkspaceDir = workspacePaths[len(workspacePaths)-1]
		} else {
			WorkspaceDir = defaultWorkspaceDir
		}
	}

	if "" != workspaceArg {
		WorkspaceDir = workspaceArg
	}

	if !gulu.File.IsDir(WorkspaceDir) {
		logging.LogWarnf("use the default workspace [%s] since the specified workspace [%s] is not a dir", defaultWorkspaceDir, WorkspaceDir)
		if err := os.MkdirAll(defaultWorkspaceDir, 0755); err != nil && !os.IsExist(err) {
			logging.LogErrorf("create default workspace folder [%s] failed: %s", defaultWorkspaceDir, err)
			os.Exit(logging.ExitCodeInitWorkspaceErr)
		}
		WorkspaceDir = defaultWorkspaceDir
	}
	workspacePaths = append(workspacePaths, WorkspaceDir)

	if err := WriteWorkspacePaths(workspacePaths); err != nil {
		logging.LogErrorf("write workspace conf [%s] failed: %s", workspaceConf, err)
		os.Exit(logging.ExitCodeInitWorkspaceErr)
	}

	WorkspaceName = filepath.Base(WorkspaceDir)
	ConfDir = filepath.Join(WorkspaceDir, "conf")
	DataDir = filepath.Join(WorkspaceDir, "data")
	RepoDir = filepath.Join(WorkspaceDir, "repo")
	HistoryDir = filepath.Join(WorkspaceDir, "history")
	TempDir = filepath.Join(WorkspaceDir, "temp")
	osTmpDir := filepath.Join(TempDir, "os")
	os.RemoveAll(osTmpDir)
	if err := os.MkdirAll(osTmpDir, 0755); err != nil {
		logging.LogErrorf("create os tmp dir [%s] failed: %s", osTmpDir, err)
		os.Exit(logging.ExitCodeInitWorkspaceErr)
	}
	os.RemoveAll(filepath.Join(TempDir, "repo"))
	os.Setenv("TMPDIR", osTmpDir)
	os.Setenv("TEMP", osTmpDir)
	os.Setenv("TMP", osTmpDir)
	DBPath = filepath.Join(TempDir, DBName)
	HistoryDBPath = filepath.Join(TempDir, "history.db")
	AssetContentDBPath = filepath.Join(TempDir, "asset_content.db")
	BlockTreeDBPath = filepath.Join(TempDir, "blocktree.db")
	SnippetsPath = filepath.Join(DataDir, "snippets")
}

func ReadWorkspacePaths() (ret []string, err error) {
	ret = []string{}
	workspaceConf := filepath.Join(HomeDir, ".config", "siyuan", "workspace.json")
	data, err := os.ReadFile(workspaceConf)
	if err != nil {
		msg := fmt.Sprintf("read workspace conf [%s] failed: %s", workspaceConf, err)
		logging.LogErrorf(msg)
		err = errors.New(msg)
		return
	}

	if err = gulu.JSON.UnmarshalJSON(data, &ret); err != nil {
		msg := fmt.Sprintf("unmarshal workspace conf [%s] failed: %s", workspaceConf, err)
		logging.LogErrorf(msg)
		err = errors.New(msg)
		return
	}

	var tmp []string
	for _, d := range ret {
		d = strings.TrimRight(d, " \t\n") // 去掉工作空间路径尾部空格 https://github.com/siyuan-note/siyuan/issues/6353
		if gulu.File.IsDir(d) {
			tmp = append(tmp, d)
		}
	}
	ret = tmp
	ret = gulu.Str.RemoveDuplicatedElem(ret)
	return
}

func WriteWorkspacePaths(workspacePaths []string) (err error) {
	workspacePaths = gulu.Str.RemoveDuplicatedElem(workspacePaths)
	workspaceConf := filepath.Join(HomeDir, ".config", "siyuan", "workspace.json")
	data, err := gulu.JSON.MarshalJSON(workspacePaths)
	if err != nil {
		msg := fmt.Sprintf("marshal workspace conf [%s] failed: %s", workspaceConf, err)
		logging.LogErrorf(msg)
		err = errors.New(msg)
		return
	}

	if err = filelock.WriteFile(workspaceConf, data); err != nil {
		msg := fmt.Sprintf("write workspace conf [%s] failed: %s", workspaceConf, err)
		logging.LogErrorf(msg)
		err = errors.New(msg)
		return
	}
	return
}

var (
	ServerURL  *url.URL // 内核服务 URL
	ServerPort = "0"    // HTTP/WebSocket 端口，0 为使用随机端口

	ReadOnly       bool
	AccessAuthCode string
	Lang           = ""

	Container        string // docker, android, ios, std
	ISMicrosoftStore bool   // 桌面端是否是微软商店版
)

const (
	ContainerStd     = "std"     // 桌面端
	ContainerDocker  = "docker"  // Docker 容器端
	ContainerAndroid = "android" // Android 端
	ContainerIOS     = "ios"     // iOS 端

	LocalHost = "127.0.0.1" // 伺服地址
	FixedPort = "6806"      // 固定端口
)

// initPathDir 初始化工作目录下的必要文件夹，包括配置文件夹、数据文件夹、临时文件夹以及数据文件夹下的子文件夹（assets, templates, widgets, plugins, emojis, public）。
// 如果文件夹创建失败且不是因为文件夹已存在，则记录致命错误并退出程序。
func initPathDir() {
	if err := os.MkdirAll(ConfDir, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create conf folder [%s] failed: %s", ConfDir, err)
	}
	if err := os.MkdirAll(DataDir, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data folder [%s] failed: %s", DataDir, err)
	}
	if err := os.MkdirAll(TempDir, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create temp folder [%s] failed: %s", TempDir, err)
	}

	assets := filepath.Join(DataDir, "assets")
	if err := os.MkdirAll(assets, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data assets folder [%s] failed: %s", assets, err)
	}

	templates := filepath.Join(DataDir, "templates")
	if err := os.MkdirAll(templates, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data templates folder [%s] failed: %s", templates, err)
	}

	widgets := filepath.Join(DataDir, "widgets")
	if err := os.MkdirAll(widgets, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data widgets folder [%s] failed: %s", widgets, err)
	}

	plugins := filepath.Join(DataDir, "plugins")
	if err := os.MkdirAll(plugins, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data plugins folder [%s] failed: %s", widgets, err)
	}

	emojis := filepath.Join(DataDir, "emojis")
	if err := os.MkdirAll(emojis, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data emojis folder [%s] failed: %s", widgets, err)
	}

	// Support directly access `data/public/*` contents via URL link https://github.com/siyuan-note/siyuan/issues/8593
	public := filepath.Join(DataDir, "public")
	if err := os.MkdirAll(public, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data public folder [%s] failed: %s", widgets, err)
	}
}

// InitPathDirByUser 根据用户编号初始化用户的目录结构。
// 该函数会创建用户的数据文件夹、临时文件夹以及其子文件夹（如 assets, templates, widgets, plugins, emojis, public）。
// 如果创建过程中发生错误且错误不是因为文件夹已存在，则会记录致命错误并退出程序。
func InitPathDirByUser(userNo string) {
	if "" == userNo {
		return
	}

	if err := os.MkdirAll(DataDir+userNo, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data folder [%s] failed: %s", DataDir, err)
	}

	assets := filepath.Join(DataDir+userNo, "assets")
	if err := os.MkdirAll(assets, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data assets folder [%s] failed: %s", assets, err)
	}

	templates := filepath.Join(DataDir+userNo, "templates")
	if err := os.MkdirAll(templates, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data templates folder [%s] failed: %s", templates, err)
	}

	widgets := filepath.Join(DataDir+userNo, "widgets")
	if err := os.MkdirAll(widgets, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data widgets folder [%s] failed: %s", widgets, err)
	}

	plugins := filepath.Join(DataDir+userNo, "plugins")
	if err := os.MkdirAll(plugins, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data plugins folder [%s] failed: %s", widgets, err)
	}

	emojis := filepath.Join(DataDir+userNo, "emojis")
	if err := os.MkdirAll(emojis, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data emojis folder [%s] failed: %s", widgets, err)
	}

	// Support directly access `data/public/*` contents via URL link https://github.com/siyuan-note/siyuan/issues/8593
	public := filepath.Join(DataDir+userNo, "public")
	if err := os.MkdirAll(public, 0755); err != nil && !os.IsExist(err) {
		logging.LogFatalf(logging.ExitCodeInitWorkspaceErr, "create data public folder [%s] failed: %s", widgets, err)
	}
}

func initMime() {
	// 在某版本的 Windows 10 操作系统上界面样式异常问题
	// https://github.com/siyuan-note/siyuan/issues/247
	// https://github.com/siyuan-note/siyuan/issues/3813
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".js", "text/javascript")
	mime.AddExtensionType(".mjs", "text/javascript")
	mime.AddExtensionType(".html", "text/html")
	mime.AddExtensionType(".json", "application/json")

	// 某些系统上下载资源文件后打开是 zip https://github.com/siyuan-note/siyuan/issues/6347
	mime.AddExtensionType(".doc", "application/msword")
	mime.AddExtensionType(".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	mime.AddExtensionType(".xls", "application/vnd.ms-excel")
	mime.AddExtensionType(".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	mime.AddExtensionType(".dwg", "image/x-dwg")
	mime.AddExtensionType(".dxf", "image/x-dxf")
	mime.AddExtensionType(".dwf", "drawing/x-dwf")
	mime.AddExtensionType(".pdf", "application/pdf")

	// 某些系统上无法显示 SVG 图片 SVG images cannot be displayed on some systems https://github.com/siyuan-note/siyuan/issues/9413
	mime.AddExtensionType(".svg", "image/svg+xml")

	// 文档数据文件
	mime.AddExtensionType(".sy", "application/json")
}

func GetDataAssetsAbsPath() (ret string) {
	ret = filepath.Join(DataDir, "assets")
	if IsSymlinkPath(ret) {
		// 跟随符号链接 https://github.com/siyuan-note/siyuan/issues/5480
		var err error
		ret, err = filepath.EvalSymlinks(ret)
		if err != nil {
			logging.LogErrorf("read assets link failed: %s", err)
		}
	}
	return
}

// tryLockWorkspace 尝试锁定工作区。如果成功锁定，则直接返回；如果锁定失败，会记录错误日志并退出程序。
// 该函数用于确保在同一时间只有一个进程能够访问工作区，防止并发操作导致的数据不一致问题。
func tryLockWorkspace() {
	WorkspaceLock = flock.New(filepath.Join(WorkspaceDir, ".lock"))
	ok, err := WorkspaceLock.TryLock()
	if ok {
		return
	}
	if err != nil {
		logging.LogErrorf("lock workspace [%s] failed: %s", WorkspaceDir, err)
	} else {
		logging.LogErrorf("lock workspace [%s] failed", WorkspaceDir)
	}
	os.Exit(logging.ExitCodeWorkspaceLocked)
}

func IsWorkspaceLocked(workspacePath string) bool {
	if !gulu.File.IsDir(workspacePath) {
		return false
	}

	lockFilePath := filepath.Join(workspacePath, ".lock")
	if !gulu.File.IsExist(lockFilePath) {
		return false
	}

	f := flock.New(lockFilePath)
	defer f.Unlock()
	ok, _ := f.TryLock()
	if ok {
		return false
	}
	return true
}

func UnlockWorkspace() {
	if nil == WorkspaceLock {
		return
	}

	if err := WorkspaceLock.Unlock(); err != nil {
		logging.LogErrorf("unlock workspace [%s] failed: %s", WorkspaceDir, err)
		return
	}

	if err := os.Remove(filepath.Join(WorkspaceDir, ".lock")); err != nil {
		logging.LogErrorf("remove workspace lock failed: %s", err)
		return
	}
}
