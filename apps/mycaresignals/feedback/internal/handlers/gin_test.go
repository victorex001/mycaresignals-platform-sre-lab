package handlers

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

func newTestGinContext(response *httptest.ResponseRecorder, request *http.Request) *gin.Context {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(response)
	context.Request = request
	return context
}
