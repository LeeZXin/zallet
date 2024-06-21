package util

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func ShouldBindYAML(req any, c *gin.Context) bool {
	err := c.ShouldBindYAML(req)
	if err != nil {
		c.String(http.StatusBadRequest, "bad request")
		return false
	}
	return true
}

func ShouldBindJSON(req any, c *gin.Context) bool {
	err := c.ShouldBindJSON(req)
	if err != nil {
		c.String(http.StatusBadRequest, "bad request")
		return false
	}
	return true
}
