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

package model

import (
	"github.com/gin-gonic/gin"
	"github.com/siyuan-note/siyuan/kernel/util"
)

const (
	UserContextKey = "userNo"
)

func IsValidUser(user string) bool {
	if user != "" {
		return true
	}
	return false
}

func GetGinContextUser(c *gin.Context) string {
	if user, exists := c.Get(UserContextKey); exists {
		return user.(string)
	} else {
		return ""
	}
}

func GetDataDir(c *gin.Context) string {
	if c != nil {
		if user, exists := c.Get(UserContextKey); exists {
			return util.DataDir + user.(string)
		} else {
			return util.DataDir
		}
	} else {
		return util.DataDir
	}
}
