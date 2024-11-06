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
	"github.com/88250/gulu"
	ginSessions "github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/siyuan-note/logging"
)

var WrongAuthCount int

func NeedCaptcha() bool {
	return 3 < WrongAuthCount
}

// SessionData represents the session.
type SessionData struct {
	Workspaces map[string]*WorkspaceSession // <WorkspacePath, WorkspaceSession>
}

type WorkspaceSession struct {
	AccessAuthCode string
	Captcha        string
}

// Save saves the current session of the specified context.
func (sd *SessionData) Save(c *gin.Context) error {
	session := ginSessions.Default(c)
	sessionDataBytes, err := gulu.JSON.MarshalJSON(sd)
	if err != nil {
		return err
	}
	session.Set("data", string(sessionDataBytes))
	return session.Save()
}

// GetSession returns session of the specified context.
// GetSession 从gin的上下文中获取会话数据。
// 如果会话数据不存在或解析失败，则返回一个新的SessionData实例。
// 成功获取并解析会话数据后，将其设置到gin上下文的"session"键中，并返回该会话数据。
func GetSession(c *gin.Context) *SessionData {
	ret := &SessionData{}

	session := ginSessions.Default(c)
	sessionDataStr := session.Get("data")
	if nil == sessionDataStr {
		return ret
	}

	err := gulu.JSON.UnmarshalJSON([]byte(sessionDataStr.(string)), ret)
	if err != nil {
		return ret
	}

	c.Set("session", ret)
	logging.LogInfof("get session: %s", ret)
	return ret
}

// GetWorkspaceSession 根据给定的 SessionData 获取或创建一个 WorkspaceSession。
// 如果 session.Workspaces 为空，则初始化一个空的 map。
// 如果 session.Workspaces 中不存在 WorkspaceDir 对应的 WorkspaceSession，则创建一个新的并存储。
// 返回对应的 WorkspaceSession。
func GetWorkspaceSession(session *SessionData) (ret *WorkspaceSession) {
	ret = &WorkspaceSession{}
	if nil == session.Workspaces {
		session.Workspaces = map[string]*WorkspaceSession{}
	}
	ret = session.Workspaces[WorkspaceDir]
	if nil == ret {
		ret = &WorkspaceSession{}
		session.Workspaces[WorkspaceDir] = ret
	}
	return
}

func RemoveWorkspaceSession(session *SessionData) {
	delete(session.Workspaces, WorkspaceDir)
}
