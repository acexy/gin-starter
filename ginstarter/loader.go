package ginstarter

import (
	"context"
	"github.com/acexy/golang-toolkit/logger"
	"github.com/acexy/golang-toolkit/util/net"
	"github.com/gin-gonic/gin"
	"github.com/golang-acexy/starter-parent/parent"
	"github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

var once sync.Once
var server *http.Server
var ginEngine *gin.Engine
var ginConfig *GinConfig

type GinConfig struct {

	// 模块组件在启动时执行初始化
	InitFunc func(instance *gin.Engine)

	// * 注册业务路由
	Routers []Router

	// * 注册服务监听地址 :8080 (默认)
	ListenAddress string // ip:port

	// 默认情况系统会将捕获的异常详细发给PanicResolver处理，如果不想将细节暴露向外
	// 方案 1. 启用隐藏异常细节功能，系统将在触发panic重要错误时不再调用PanicResolver处理，并统一响应500错误
	// 方案 2. 如果不想禁用异常时调用PanicResolver, 可以在初始化时手动设置自定义PanicResolver处理器
	// * panic 将被分为框架内部错误和框架未知错误 框架内部错误是非敏感错误，不受该参数控制，每次都会触发PanicResolver，例如验证框架错误
	HidePanicErrorDetails bool
	// 全局异常响应处理器 如果不指定则使用默认方式
	PanicResolver PanicResolver

	// 禁用异常http响应码Resolver
	DisableBadHttpCodeResolver bool
	// 禁用系统内置的忽略异常响应码
	DisableDefaultIgnoreHttpCode bool
	// 启用异常http响应码Resolver 指定不处理特定的异常响应码
	IgnoreHttpCode []int
	// 启用异常http响应码Resolver 如果不指定则使用默认方式
	BadHttpCodeResolver BadHttpCodeResolver

	// 自定义全局中间件 作用于所有请求 按照顺序执行
	GlobalMiddlewares []Middleware

	// 响应数据的结构体解码器 默认为JSON方式解码
	// 在使用NewRespRest响应结构体数据时解码为[]byte数据的解码器
	// 如果自实现Response接口将不使用解码器
	ResponseDataStructDecoder ResponseDataStructDecoder

	// 尝试启用TraceId响应
	// https://github.com/acexy/golang-toolkit/blob/main/sys/threadlocal.go
	// 如果工作环境开启EnableLocalTraceId ，将自动响应TranceId头
	EnableGoroutineTraceIdResponse bool

	// ========== gin config
	DebugModule        bool
	MaxMultipartMemory int64

	// 关闭包裹405错误展示，使用404代替
	DisableMethodNotAllowedError bool

	// 禁用尝试获取转发真实IP
	DisableForwardedByClientIP bool
}

type GinStarter struct {

	// GinConfig 配置
	Config GinConfig
	// 懒加载函数，用于在实际执行时动态获取配置 该权重高于Config的直接配置
	LazyConfig func() GinConfig
	// 自定义Gin模块的组件属性
	GinSetting *parent.Setting
}

// 获取配置信息
func (g *GinStarter) getConfig() *GinConfig {
	once.Do(func() {
		if g.LazyConfig != nil {
			config := g.LazyConfig()
			ginConfig = &config
		} else {
			ginConfig = &g.Config
		}
	})
	return ginConfig
}

func (g *GinStarter) Setting() *parent.Setting {
	if g.GinSetting != nil {
		return g.GinSetting
	}
	config := g.getConfig()
	return parent.NewSetting(
		"Gin-Starter",
		0,
		false,
		time.Second*30,
		func(instance interface{}) {
			if config.InitFunc != nil {
				config.InitFunc(instance.(*gin.Engine))
			}
		})
}

func (g *GinStarter) Start() (interface{}, error) {
	var err error
	config := g.getConfig()
	if config.DebugModule {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	gin.DefaultWriter = &logrusLogger{log: logger.Logrus(), level: logrus.DebugLevel}
	gin.DefaultErrorWriter = &logrusLogger{log: logger.Logrus(), level: logrus.ErrorLevel}
	ginEngine = gin.New()
	registerValidators()
	ginEngine.Use(recoverHandler())

	if config.PanicResolver == nil {
		config.PanicResolver = panicResolver
	}

	if config.MaxMultipartMemory > 0 {
		ginEngine.MaxMultipartMemory = config.MaxMultipartMemory
	}

	ginEngine.ForwardedByClientIP = !config.DisableForwardedByClientIP

	if !config.DisableMethodNotAllowedError {
		ginEngine.HandleMethodNotAllowed = true
	}

	if !config.DisableBadHttpCodeResolver {
		ginEngine.Use(responseRewriteHandler())
		if config.BadHttpCodeResolver == nil {
			config.BadHttpCodeResolver = badHttpCodeResolver
		}
	}

	if config.ResponseDataStructDecoder == nil {
		config.ResponseDataStructDecoder = responseJsonDataStructDecoder{}
	}

	if len(config.GlobalMiddlewares) > 0 {
		for i := range config.GlobalMiddlewares {
			middleware := config.GlobalMiddlewares[i]
			if middleware != nil {
				ginEngine.Use(func(ctx *gin.Context) {
					response, continued := middleware(&Request{ctx: ctx})
					if !continued {
						httpResponse(ctx, response)
						ctx.Abort()
					} else {
						ctx.Next()
					}
				})
			}
		}
	}

	if len(config.Routers) > 0 {
		registerRouter(ginEngine, config.Routers)
	}

	if config.ListenAddress == "" {
		config.ListenAddress = ":8080"
	}

	server = &http.Server{
		Addr:    config.ListenAddress,
		Handler: ginEngine,
	}

	errChn := make(chan error)
	go func() {
		if err = server.ListenAndServe(); err != nil {
			errChn <- err
		}
	}()

	select {
	case <-time.After(time.Second):
		return ginEngine, nil
	case err = <-errChn:
		return ginEngine, err
	}
}

func (g *GinStarter) Stop(maxWaitTime time.Duration) (gracefully, stopped bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
	defer cancel()
	if err = server.Shutdown(ctx); err != nil {
		gracefully = false
	} else {
		gracefully = true
	}
	stopped = !net.Telnet(g.getConfig().ListenAddress, time.Second)
	return
}

// RawGinEngine 获取原始的gin引擎实例
func RawGinEngine() *gin.Engine {
	return ginEngine
}
