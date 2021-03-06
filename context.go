package xingyun

import (
	"fmt"
	"net/http"

	"github.com/gorilla/context"
)

const (
	CONTEXT_KEY = "_XINGYUN_CONTEXT_"
)

var (
	DefaultFormMaxMemmory int64 = 64 << 20
)

type ContextHandler interface {
	ServeContext(ctx *Context)
}

type ContextHandlerFunc func(ctx *Context)

func (h ContextHandlerFunc) ServeContext(ctx *Context) {
	h(ctx)
}

func ToHTTPHandlerFunc(h ContextHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ServeContext(getUnInitedContext(r, w))
	}
}

func FromHTTPHandlerFunc(h http.HandlerFunc) ContextHandlerFunc {
	return func(ctx *Context) {
		h.ServeHTTP(ctx.ResponseWriter, ctx.Request)
	}
}

func ToHTTPHandler(h ContextHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeContext(getUnInitedContext(r, w))
	})
}

func FromHTTPHandler(h http.Handler) ContextHandler {
	return ContextHandlerFunc(func(ctx *Context) {
		h.ServeHTTP(ctx.ResponseWriter, ctx.Request)
	})
}

func wrapResponseWriter(w http.ResponseWriter) ResponseWriter {
	rw, ok := w.(ResponseWriter)
	if ok {
		return rw
	}
	return NewResponseWriter(w)
}

type SessionStorage interface {
	SetSession(sessionID string, key string, data []byte)
	GetSession(sessionID string, key string) []byte
	ClearSession(sessionID string, key string)
}

type Context struct {
	ResponseWriter
	Request *http.Request
	Server  *Server
	Config  *Config

	Logger Logger

	Params map[string]string

	IsPanic      bool
	PanicError   interface{}
	StackMessage string

	// use for user ContextHandler
	Data map[string]interface{}
	// use for user PipeHandler. avoid name conflict
	PipeHandlerData map[string]interface{}

	isInited   bool
	flash      *Flash
	staticData map[string][]string
	opts       *xsrfOptions
	xsrf       *xsrf
}

func GetContext(r *http.Request) *Context {
	obj, ok := context.GetOk(r, CONTEXT_KEY)
	if !ok {
		panic(fmt.Errorf("can't get context, &r=%p", r))
	}
	ctx := obj.(*Context)
	if !ctx.isInited {
		panic(fmt.Errorf("get uninited context, &r=%p", r))
	}
	return ctx
}

func initContext(r *http.Request, w http.ResponseWriter, s *Server) *Context {
	ctx := getUnInitedContext(r, w)
	if ctx.isInited {
		return ctx
	}
	*ctx = Context{
		ResponseWriter: wrapResponseWriter(w),
		Request:        r,
		Server:         s,
		Config:         s.Config,
		Logger:         s.logger,
		Params:         map[string]string{},
		Data:           map[string]interface{}{},
		staticData:     map[string][]string{},
	}
	ctx.parseParams()
	ctx.isInited = true
	context.Set(r, CONTEXT_KEY, ctx)
	s.logger.Debugf("init context, &r=%p", r)
	return ctx
}

func getUnInitedContext(r *http.Request, w http.ResponseWriter) *Context {
	ctx, ok := context.GetOk(r, CONTEXT_KEY)
	if !ok {
		newctx := &Context{Request: r, ResponseWriter: wrapResponseWriter(w)}
		context.Set(r, CONTEXT_KEY, newctx)
		return newctx
	}
	return ctx.(*Context)
}

func (ctx *Context) parseParams() {
	var err error
	err = ctx.Request.ParseMultipartForm(DefaultFormMaxMemmory)
	if err != nil && err.Error() != http.ErrNotMultipart.Error() {
		ctx.Logger.Errorf(err.Error())
		return
	}
	for k, v := range ctx.Request.Form {
		ctx.Params[k] = v[0]
	}
}

func (ctx *Context) checkHeaderWrite() {
	if ctx.ResponseWriter.Written() {
		panic(fmt.Errorf("must write header before body"))
	}
}
