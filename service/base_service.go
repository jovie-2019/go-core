package service

import (
	"errors"
	"fmt"
	"github.com/iris-contrib/middleware/cors"
	"github.com/kataras/iris"
	"github.com/kataras/iris/context"
	"github.com/pefish/go-application"
	"github.com/pefish/go-core/api-channel-builder"
	"github.com/pefish/go-core/api-session"
	"github.com/pefish/go-core/middlewares"
	"github.com/pefish/go-core/validator"
	"github.com/pefish/go-error"
	"github.com/pefish/go-format"
	"github.com/pefish/go-http"
	"github.com/pefish/go-logger"
	"github.com/pefish/go-reflect"
	"reflect"
	"time"
)

var (
	jwtHeaderName = `Json-Web-Token`
)

type BaseServiceClass struct {
	Name              string                                        // 服务名
	Description       string                                        // 服务描述
	Path              string                                        // 服务的基础路径
	Host              string                                        // 服务监听host
	Port              string                                        // 服务监听port
	AccessHost        string                                        // 服务访问host，没有设置的话使用监听host
	AccessPort        string                                        // 服务访问port，没有设置的话使用监听port
	Routes            map[string]*api_session.Route                 // 服务的所有路由
	Middlewires       map[string]api_channel_builder.InjectFuncType // 每个api的前置处理器（框架的）
	GlobalMiddlewires map[string]context.Handler                    // 每个api的前置处理器（iris的）
	App               *iris.Application                             // iris实例
	HealthyCheckFun   func()                                        // health check 控制器
	Opts              map[string]interface{}                        // 一些可选参数
}

func (this *BaseServiceClass) Init(opts ...interface{}) InterfaceService {
	return this
}

func (this *BaseServiceClass) SetHealthyCheck(func_ func()) InterfaceService {
	this.HealthyCheckFun = func_
	return this
}

func (this *BaseServiceClass) Use(key string, func_ api_channel_builder.InjectFuncType) InterfaceService {
	if this.Middlewires == nil {
		this.Middlewires = map[string]api_channel_builder.InjectFuncType{}
	}
	this.Middlewires[key] = func_
	return this
}

func (this *BaseServiceClass) UseGlobal(key string, func_ context.Handler) InterfaceService {
	if this.GlobalMiddlewires == nil {
		this.GlobalMiddlewires = map[string]context.Handler{}
	}
	this.GlobalMiddlewires[key] = func_
	return this
}

func (this *BaseServiceClass) GetName() string {
	return this.Name
}

func (this *BaseServiceClass) GetDescription() string {
	return this.Description
}

func (this *BaseServiceClass) GetPath() string {
	return this.Path
}

func (this *BaseServiceClass) GetRoutes() map[string]*api_session.Route {
	return this.Routes
}

func (this *BaseServiceClass) ExactOpt(name string) interface{} {
	if name == `jwt_header_name` && (this.Opts == nil || this.Opts[name] == nil) {
		return `Json-Web-Token`
	} else {
		return this.Opts[name]
	}
}

func (this *BaseServiceClass) RequestWithErr(apiName string, args ...interface{}) (interface{}, error) {
	body := this.RequestRawMap(apiName, args...)
	if !body[`succeed`].(bool) {
		errorMessage := p_error.INTERNAL_ERROR
		if body[`error_message`] != nil {
			errorMessage = body[`error_message`].(string)
		}
		return body, errors.New(errorMessage)
	}
	return body[`data`], nil
}

func (this *BaseServiceClass) RequestRawMap(apiName string, args ...interface{}) map[string]interface{} {
	var params interface{}
	if len(args) > 0 && args[0] != nil {
		params = args[0]
	} else {
		params = map[string]interface{}{}
	}

	headers := map[string]string{}
	// header内容转发
	if len(args) > 1 && args[1] != nil {
		if apiSession, ok := args[1].(*api_session.ApiSessionClass); ok {
			jwtHeaderName := p_reflect.Reflect.ToString(this.ExactOpt(`jwt_header_name`))
			headers = map[string]string{
				`lang`:            apiSession.Lang,
				`client_type`:     apiSession.ClientType,
				jwtHeaderName:     apiSession.Ctx.GetHeader(jwtHeaderName),
				`X-Forwarded-For`: apiSession.Ctx.GetHeader(`X-Forwarded-For`),
			}
		}
	}
	if this.Routes[apiName] == nil {
		p_error.Throw(`api request 404`, p_error.CODE_ERROR)
	}
	method := this.Routes[apiName].Method
	fullUrl := this.GetRequestUrl(apiName)
	body := map[string]interface{}{}
	if method == `GET` {
		body = p_http.Http.GetWithParamsForMap(fullUrl, params, headers)
	} else if method == `POST` {
		body = p_http.Http.PostForMap(fullUrl, params, headers)
	} else {
		p_error.Throw(`request not support method`, p_error.CODE_ERROR)
	}
	return body
}

/**
http请求其他服务。
apiName：请求哪个api
args：args[0]是参数，可以是struct或者map；args[1]是ApiSessionClass，如果存在则会转发一些预制header；
*/
func (this *BaseServiceClass) Request(apiName string, args ...interface{}) (data interface{}) {
	body := this.RequestRawMap(apiName, args...)
	if !body[`succeed`].(bool) {
		errorMessage := p_error.INTERNAL_ERROR
		if body[`error_message`] != nil {
			errorMessage = body[`error_message`].(string)
		}
		p_error.ThrowErrorWithData(errorMessage, p_reflect.Reflect.ToUint64(body[`error_code`]), body[`data`], nil)
	}
	data = body[`data`]
	return
}

func (this *BaseServiceClass) RequestForSlice(apiName string, args ...interface{}) []map[string]interface{} {
	requestResult := this.Request(apiName, args...)
	if requestResult == nil {
		return []map[string]interface{}{}
	}
	return p_format.Format.SliceInterfaceToSliceMapInterface(requestResult.([]interface{}))
}

func (this *BaseServiceClass) RequestForSliceWithScan(dest interface{}, apiName string, args ...interface{}) {
	requestResult := this.Request(apiName, args...)
	if requestResult == nil {
		dest = nil
	}
	p_format.Format.SliceToStruct(dest, requestResult)
}

func (this *BaseServiceClass) RequestForMap(apiName string, args ...interface{}) map[string]interface{} {
	requestResult := this.Request(apiName, args...)
	if requestResult == nil {
		return map[string]interface{}{}
	}
	return requestResult.(map[string]interface{})
}

func (this *BaseServiceClass) RequestForMapWithScan(dest interface{}, apiName string, args ...interface{}) {
	requestResult := this.Request(apiName, args...)
	if requestResult == nil {
		dest = nil
	}
	p_format.Format.MapToStruct(dest, requestResult)
}

func (this *BaseServiceClass) Run() {
	this.buildRoutes()
	irisConfig := iris.Configuration{}
	irisConfig.RemoteAddrHeaders = map[string]bool{
		`X-Forwarded-For`: true,
	}
	err := this.App.Run(iris.Addr(this.Host+`:`+this.Port), iris.WithConfiguration(irisConfig))
	if err != nil {
		panic(err)
	}
}

func (this *BaseServiceClass) buildRoutes() {
	this.App = iris.New()
	if p_application.Application.Debug {
		this.App.UseGlobal(cors.New(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowCredentials: true,
			AllowedHeaders:   []string{`*`},
			AllowedMethods:   []string{`PUT`, `POST`, `GET`, `DELETE`, `OPTIONS`},
			Debug:            p_application.Application.Debug,
		}))
	}
	this.App.UseGlobal(middlewares.ErrorHandle)
	this.App.UseGlobal(middlewares.OptionHandle)
	for _, fun := range this.GlobalMiddlewires {
		this.App.UseGlobal(fun)
	}

	for name, route := range this.GetRoutes() {
		var apiChannelBuilder = api_channel_builder.NewApiChannelBuilder()
		// 注入一些预定义函数
		apiChannelBuilder = apiChannelBuilder.RouteBaseInfo(name, route)
		apiChannelBuilder.Inject(`header`, func(ctx iris.Context, out *api_session.ApiSessionClass) {
			lang := ctx.GetHeader(`lang`)
			if lang == `` {
				lang = `zh-CN`
			}
			out.Lang = lang

			clientType := ctx.GetHeader(`client_type`)
			if clientType == `` {
				clientType = `web`
			}
			out.ClientType = clientType
		})
		for key, fun := range this.Middlewires {
			apiChannelBuilder.Inject(key, fun)
		}
		if route.Strategies != nil {
			for _, slice_ := range route.Strategies {
				if slice_[0] == `param_validate` {
					myValidator := validator.ValidatorClass{}
					myValidator.Init()
					apiChannelBuilder = apiChannelBuilder.ParamValidate(myValidator.Validator)
				} else if slice_[0] == `jwt_auth` {
					if len(slice_) < 2 {
						p_error.ThrowInternal(`jwt_auth config error`)
					}
					apiChannelBuilder = apiChannelBuilder.JwtAuth(jwtHeaderName, slice_[1])
				} else if slice_[0] == `rate_limit` {
					if len(slice_) < 2 {
						p_error.ThrowInternal(`rate_limit config error`)
					}
					apiChannelBuilder = apiChannelBuilder.RateLimit(slice_[1].(time.Duration))
				}
			}
		}
		if route.Controller == nil {
			if route.Redirect != nil { // 自动转发。不会校验参数
				redirectMap := route.Redirect
				return_ := this.parseReturn(route.Return)
				method := `ALL`
				if route.Method != `` {
					method = route.Method
				}
				this.App.AllowMethods(iris.MethodOptions).Handle(method, this.Path+route.Path, apiChannelBuilder.WrapJson(func(apiContext *api_session.ApiSessionClass) interface{} {
					params := map[string]string{}
					apiContext.ScanParams(&params)
					service := redirectMap[`service`].(InterfaceService)
					routeName := redirectMap[`route_name`].(string)
					if service.GetRoutes()[routeName] == nil && return_ != nil { // 目标服务路由不存在，则返回规定的返回值(自动mock)
						return return_
					}
					return service.Request(routeName, params, apiContext) // 自动定位目标api的method
				}))
			} else { // 自动mock
				return_ := this.parseReturn(route.Return)
				if return_ == nil {
					p_error.ThrowInternal(`route config error; route name: ` + name)
				}
				this.App.AllowMethods(iris.MethodOptions).Handle(route.Method, this.Path+route.Path, apiChannelBuilder.WrapJson(func(apiContext *api_session.ApiSessionClass) interface{} {
					return return_
				}))
			}
		} else {
			this.App.AllowMethods(iris.MethodOptions).Handle(route.Method, this.Path+route.Path, apiChannelBuilder.WrapJson(route.Controller))
		}
	}

	this.App.AllowMethods(iris.MethodOptions).Handle(``, `/healthz`, func(ctx context.Context) {
		defer func() {
			if err := recover(); err != nil {
				p_logger.Logger.Error(err)
				ctx.StatusCode(iris.StatusInternalServerError)
				ctx.Text(`not ok`)
			}
		}()
		if this.HealthyCheckFun != nil {
			this.HealthyCheckFun()
		}

		ctx.StatusCode(iris.StatusOK)
		if p_application.Application.Debug {
			p_logger.Logger.Info(`I am healthy`)
		}
		ctx.Text(`ok`)
	})

	this.App.AllowMethods(iris.MethodOptions).Handle(``, `/*`, func(ctx context.Context) {
		ctx.StatusCode(iris.StatusNotFound)
		if p_application.Application.Debug {
			p_logger.Logger.Info(`api not found`)
		}
		ctx.Text(`Not Found`)
	})
}

func (this *BaseServiceClass) recurStruct(type_ reflect.Type, result map[string]interface{}) {
	for i := 0; i < type_.NumField(); i++ {
		field := type_.Field(i)
		fieldType := field.Type
		if fieldType.Kind() == reflect.Struct {
			this.recurStruct(fieldType, result)
		} else {
			tagName := field.Tag.Get(`example`)
			if tagName != `` {
				result[field.Tag.Get(`json`)] = tagName
			} else {
				result[field.Tag.Get(`json`)] = nil
			}
		}
	}
}

func (this *BaseServiceClass) parseReturn(return_ interface{}) interface{} {
	var result interface{}
	if return_ == nil {
		return nil
	}
	type_ := reflect.TypeOf(return_)
	kind := type_.Kind()
	if kind == reflect.Map {
		map_ := return_.(map[string]map[string]interface{})
		resultTemp := map[string]interface{}{}
		for key, obj := range map_ {
			resultTemp[key] = obj[`example`]
		}
		result = resultTemp
	} else if kind == reflect.Struct {
		resultTemp := map[string]interface{}{}
		this.recurStruct(type_, resultTemp)
		result = resultTemp
	} else if kind == reflect.Slice {
		resultTemp := []map[string]interface{}{}
		eleType := type_.Elem()
		if eleType.Kind() == reflect.Struct {
			tempMap := map[string]interface{}{}
			this.recurStruct(eleType, tempMap)
			resultTemp = append(resultTemp, tempMap)
		} else if type_.Elem().Kind() == reflect.Map {
			slice_ := return_.([]map[string]map[string]interface{})
			for _, map_ := range slice_ {
				tempMap := map[string]interface{}{}
				for key, obj := range map_ {
					tempMap[key] = obj[`example`]
				}
				resultTemp = append(resultTemp, tempMap)
			}
		} else {
			p_error.ThrowInternal(`route return error`)
		}
		result = resultTemp
	} else {
		p_error.ThrowInternal(`route return error`)
	}
	return result
}

func (this *BaseServiceClass) GetRequestUrl(apiName string) string {
	if this.Routes[apiName].Debug == true {
		return this.Routes[apiName].Path
	}
	host := this.Host
	if this.AccessHost != `` {
		host = this.AccessHost
	}
	port := this.Port
	if this.AccessPort != `` {
		port = this.AccessPort
	}
	return fmt.Sprintf(`http://%s:%s%s%s`, host, port, this.Path, this.Routes[apiName].Path)
}