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

//go:build !mobile

package main

// 导入了一系列用于构建 Siyuan Note 内核的包，包括缓存、任务处理、模型定义、服务器实现、SQL 操作和实用工具。这些包共同支持了 Siyuan Note 的核心功能。
import (
	"github.com/siyuan-note/siyuan/kernel/cache"
	"github.com/siyuan-note/siyuan/kernel/job"
	"github.com/siyuan-note/siyuan/kernel/model"
	"github.com/siyuan-note/siyuan/kernel/server"
	"github.com/siyuan-note/siyuan/kernel/sql"
	"github.com/siyuan-note/siyuan/kernel/util"
)

// main 函数是程序的入口点，负责初始化配置、启动服务、加载数据等操作。
// 该函数首先调用 util.Boot() 进行基础启动工作，然后初始化模型配置和外观。
// 接着启动服务器，并初始化数据库连接。设置搜索相关的配置。
// 同步数据，初始化盒子，加载闪卡和资产文本。
// 设置启动标志，推送清除所有消息的通知。
// 启动定时任务，自动生文件历史记录，加载资产到缓存，检查文件系统状态。
// 监视资产和表情的变化，处理系统信号。
func main() {
	util.Boot()

	model.InitConf()
	go server.Serve(false)
	model.InitAppearance()
	sql.InitDatabase(false)
	sql.InitHistoryDatabase(false)
	sql.InitAssetContentDatabase(false)
	sql.SetCaseSensitive(model.Conf.Search.CaseSensitive)
	sql.SetIndexAssetPath(model.Conf.Search.IndexAssetPath)

	model.BootSyncData()
	model.InitBoxes(nil)
	model.LoadFlashcards()
	util.LoadAssetsTexts()

	util.SetBooted()
	util.PushClearAllMsg()

	job.StartCron()
	go model.AutoGenerateFileHistory()
	go cache.LoadAssets()
	go util.CheckFileSysStatus()

	model.WatchAssets()
	model.WatchEmojis()
	model.HandleSignal()
}
