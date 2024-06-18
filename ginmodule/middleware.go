package ginmodule

import (
	"github.com/acexy/golang-toolkit/logger"
	"github.com/gin-gonic/gin"
	"net/http"
)

var (

	// 默认处理状态码handler的响应执行器
	defaultHttpStatusCodeHandlerResponse HttpStatusCodeCodeHandlerResponse = func(ctx *gin.Context, httpStatusCode int) Response {
		logger.Logrus().Warning("Request bad response http status code", ctx.Request.URL, httpStatusCode)
		v, ok := httpCodeWithStatus[httpStatusCode]
		if !ok {
			return RespRestStatusError(StatusCodeException)
		} else {
			return RespRestStatusError(v)
		}
	}

	defaultRecoverHandlerResponse RecoverHandlerResponse = func(ctx *gin.Context, err any) Response {
		logger.Logrus().Errorln("Request catch exception", ctx.Request.URL, "panic: ", err)
		return RespRestException()
	}

	httpCodeWithStatus map[int]StatusCode
)

type HttpStatusCodeCodeHandlerResponse func(ctx *gin.Context, httpStatusCode int) Response
type RecoverHandlerResponse func(ctx *gin.Context, err any) Response

func init() {
	httpCodeWithStatus = make(map[int]StatusCode, 6)
	httpCodeWithStatus[http.StatusBadRequest] = StatusCodeBadRequestParameters
	httpCodeWithStatus[http.StatusForbidden] = StatusCodeForbidden
	httpCodeWithStatus[http.StatusNotFound] = StatusCodeNotFound
	httpCodeWithStatus[http.StatusMethodNotAllowed] = StatusCodeMethodNotAllowed
	httpCodeWithStatus[http.StatusUnsupportedMediaType] = StatusCodeMediaTypeNotAllowed
	httpCodeWithStatus[http.StatusRequestEntityTooLarge] = StatusCodeUploadLimitExceeded
	httpCodeWithStatus[http.StatusUnauthorized] = StatusCodeForbidden
}

// RecoverHandler 全局Panic处理中间件
func RecoverHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				response := defaultRecoverHandlerResponse(ctx, r)
				if response != nil {
					httpResponse(ctx, response)
				}
			}
		}()
		ctx.Next()
	}
}

// RewriteHttpStatusCodeHandler 可重写Http状态码中间件
func RewriteHttpStatusCodeHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		writer := &responseStatusRewriter{
			ctx.Writer,
			http.StatusNotFound,
		}
		ctx.Writer = writer
		ctx.Next()
		ctx.Writer.WriteHeader(writer.statusCode)
	}
}

// HttpStatusCodeHandler 异常状态码自动转换响应中间件
func HttpStatusCodeHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
		statusCode := ctx.Writer.Status()
		if statusCode != http.StatusOK {
			ctx.Writer.WriteHeader(http.StatusOK)
			response := defaultHttpStatusCodeHandlerResponse(ctx, statusCode)
			httpResponse(ctx, response)
		}
	}
}
