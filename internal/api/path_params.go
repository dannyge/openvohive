package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func firstPathParam(c *gin.Context, names ...string) string {
	if c == nil {
		return ""
	}
	for _, name := range names {
		if v := strings.TrimSpace(c.Param(name)); v != "" {
			return v
		}
	}
	return ""
}

func deviceIDParam(c *gin.Context) string {
	return firstPathParam(c, "device_id", "id")
}


