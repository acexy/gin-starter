package ginmodule

import (
	"github.com/acexy/golang-toolkit/logger"
	"github.com/gin-gonic/gin"
	"net/http"
)

type BasicAuthAccount struct {
	Username string
	Password string
}

type RouterInfo struct {
	// GroupPath 路由分组路径
	GroupPath string

	// BasicAuthAccount 如果指定基于BasicAuth认证的账户，则该GroupPath下资源将需要权限认证
	BasicAuthAccount *BasicAuthAccount
}

// RouterWrapper 定义路由包装器
type RouterWrapper struct {
	routerGroup *gin.RouterGroup
}

// HandlerWrapper 定义内部Handler
type HandlerWrapper func(request *Request) (Response, error)

type Router interface {
	// Info 定义路由信息
	Info() *RouterInfo

	// Handlers 注册处理器
	Handlers(router *RouterWrapper)
}

// 定义RouterWrapper的接收请求行为

func (r *RouterWrapper) POST(path string, handler ...HandlerWrapper) {
	r.handler([]string{http.MethodPost}, path, handler...)
}

func (r *RouterWrapper) GET(path string, handler ...HandlerWrapper) {
	r.handler([]string{http.MethodGet}, path, handler...)
}

func (r *RouterWrapper) HEAD(path string, handler ...HandlerWrapper) {
	r.handler([]string{http.MethodHead}, path, handler...)
}

func (r *RouterWrapper) PUT(path string, handler ...HandlerWrapper) {
	r.handler([]string{http.MethodPut}, path, handler...)
}

func (r *RouterWrapper) PATCH(path string, handler ...HandlerWrapper) {
	r.handler([]string{http.MethodPatch}, path, handler...)
}

func (r *RouterWrapper) DELETE(path string, handler ...HandlerWrapper) {
	r.handler([]string{http.MethodDelete}, path, handler...)
}

func (r *RouterWrapper) OPTIONS(path string, handler ...HandlerWrapper) {
	r.handler([]string{http.MethodOptions}, path, handler...)
}

func (r *RouterWrapper) TRACE(path string, handler ...HandlerWrapper) {
	r.handler([]string{http.MethodTrace}, path, handler...)
}

func (r *RouterWrapper) MATCH(method []string, path string, handler ...HandlerWrapper) {
	r.handler(method, path, handler...)
}

// 执行RouterWrapper行为

func (r *RouterWrapper) handler(methods []string, path string, handlerWrapper ...HandlerWrapper) {
	handlers := make([]gin.HandlerFunc, len(handlerWrapper))
	for i, handler := range handlerWrapper {
		handlers[i] = func(context *gin.Context) {
			if context.IsAborted() {
				logger.Logrus().Warning("Request is aborted")
				return
			}

			response, err := handler(&Request{context})
			if err != nil {
				panic(err)
			}

			if response != nil {
				httpResponse(context, response)
			} else {
				context.Status(http.StatusOK)
			}
		}
	}
	r.routerGroup.Match(methods, path, handlers...)
}

func httpResponse(context *gin.Context, response Response) {

	// 如果是普通响应 判断是否使用了gin原始响应功能
	if instance, ok := response.(commonResp); ok {
		if instance.ginFn != nil {
			instance.ginFn(context)
			return
		}
	}

	responseData := response.Data()
	if responseData == nil {
		return
	}

	contentType := responseData.contentType
	if contentType == "" {
		contentType = gin.MIMEJSON
		logger.Logrus().Traceln("ContentType is not set, use default", gin.MIMEJSON)
	}

	httpStatusCode := responseData.statusCode
	if httpStatusCode == 0 {
		httpStatusCode = http.StatusOK
	}

	cookies := responseData.cookies
	if len(cookies) > 0 {
		for _, v := range cookies {
			context.SetCookie(v.name, v.value, v.maxAge, v.path, v.domain, v.secure, v.httpOnly)
		}
	}

	headers := responseData.headers
	if len(headers) > 0 {
		for _, v := range headers {
			context.Header(v.name, v.value)
		}
	}

	data := responseData.data
	if len(data) > 0 {
		context.Data(httpStatusCode, contentType, data)
	}
}

// 支持将gin statusCode重写的响应处理器
type responseStatusRewriter struct {
	gin.ResponseWriter
	statusCode int
}

func (r *responseStatusRewriter) WriteHeader(code int) {
	r.statusCode = code
}

func (r *responseStatusRewriter) Write(data []byte) (int, error) {
	if !r.Written() {
		r.ResponseWriter.WriteHeader(r.statusCode)
	}
	return r.ResponseWriter.Write(data)
}

func (r *responseStatusRewriter) WriteHeaderNow() {
	if !r.Written() {
		r.ResponseWriter.WriteHeader(r.statusCode)
	}
}

func (r *responseStatusRewriter) Status() int {
	return r.statusCode
}

// ResponseData 标准响应数据内容
type ResponseData struct {
	// body响应体负载数据
	data []byte
	// ContentType 响应的ContentType
	contentType string
	// 响应状态码
	statusCode int
	// 响应头
	headers []*ResponseHeader
	// 响应Cookie
	cookies []*ResponseCookie
}

// ResponseHeader 响应头
type ResponseHeader struct {
	name string
	// 设置零值可以清除该Name响应头
	value string
}

// ResponseCookie 响应Cookie
type ResponseCookie struct {
	name     string
	value    string
	maxAge   int
	path     string
	domain   string
	secure   bool
	httpOnly bool
}

func NewResponseData() *ResponseData {
	return &ResponseData{}
}

func NewHeader(name, value string) *ResponseHeader {
	return &ResponseHeader{name: name, value: value}
}

func NewCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) *ResponseCookie {
	return &ResponseCookie{name: name, value: value, maxAge: maxAge, path: path, domain: domain, secure: secure, httpOnly: httpOnly}
}

func (r *ResponseData) SetData(data []byte) *ResponseData {
	r.data = data
	return r
}

func (r *ResponseData) SetContentType(contentType string) *ResponseData {
	r.contentType = contentType
	return r
}

func (r *ResponseData) SetStatusCode(statusCode int) *ResponseData {
	r.statusCode = statusCode
	return r
}

func (r *ResponseData) AddHeaders(headers []*ResponseHeader) *ResponseData {
	if len(r.headers) != 0 {
		r.headers = append(r.headers, headers...)
	} else {
		r.headers = headers
	}
	return r
}

func (r *ResponseData) AddHeader(header *ResponseHeader) *ResponseData {
	if len(r.headers) == 0 {
		r.headers = []*ResponseHeader{header}
	} else {
		r.headers = append(r.headers, header)
	}
	return r
}

func (r *ResponseData) AddCookies(cookies []*ResponseCookie) *ResponseData {
	if len(r.cookies) != 0 {
		r.cookies = append(r.cookies, cookies...)
	} else {
		r.cookies = cookies
	}
	return r
}

func (r *ResponseData) AddCookie(cookie *ResponseCookie) *ResponseData {
	if len(r.cookies) == 0 {
		r.cookies = []*ResponseCookie{cookie}
	} else {
		r.cookies = append(r.cookies, cookie)
	}
	return r
}
