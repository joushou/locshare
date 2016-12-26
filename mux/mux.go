package mux

import (
	"context"
	"log"
	"net/http"
	"strings"
)

var notFound = http.NotFoundHandler()

type Mux struct {
	other http.Handler
	mux   map[string]http.Handler
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == "" {
		p = "/"
	}

	part := p
	remainder := ""
	idx := strings.Index(p[1:], "/")
	if idx != -1 {
		part = p[:idx+1]
		remainder = p[idx+1:]
	}

	h := m.mux[part]
	if h == nil {
		if m.other != nil {
			m.other.ServeHTTP(w, r)
			return
		}

		http.NotFound(w, r)
		return
	}

	u := *r.URL
	u.Path = remainder
	r2 := new(http.Request)
	*r2 = *r
	r2.URL = &u

	h.ServeHTTP(w, r2)
}

func (m *Mux) Otherwise(f http.Handler) *Mux {
	m.other = f
	return m
}

func (m *Mux) OtherwiseFunc(f http.HandlerFunc) *Mux {
	m.other = f
	return m
}

func (m *Mux) Handle(p string, f http.Handler) *Mux {
	m.mux[p] = f
	return m
}

func (m *Mux) HandleFunc(p string, f http.HandlerFunc) *Mux {
	m.mux[p] = f
	return m
}

func New() *Mux {
	return &Mux{
		mux: make(map[string]http.Handler),
	}
}

type ParamMux struct {
	key     interface{}
	noParam http.Handler
	param   http.Handler
}

func (m *ParamMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == "" || p == "/" {
		if m.noParam == nil {
			http.NotFound(w, r)
			return
		}

		m.noParam.ServeHTTP(w, r)
		return
	}

	if m.param == nil {
		http.NotFound(w, r)
		return
	}

	param := p[1:]
	remainder := ""
	idx := strings.Index(p[1:], "/")
	if idx != -1 {
		param = p[1 : idx+1]
		remainder = p[idx+1:]
	}

	ctx := context.WithValue(r.Context(), m.key, param)
	r = r.WithContext(ctx)

	u := *r.URL
	u.Path = remainder
	r2 := new(http.Request)
	*r2 = *r
	r2.URL = &u

	m.param.ServeHTTP(w, r2)
}

func (m *ParamMux) Param(h http.Handler) *ParamMux {
	m.param = h
	return m
}

func (m *ParamMux) ParamFunc(f http.HandlerFunc) *ParamMux {
	m.param = f
	return m
}

func (m *ParamMux) NoParam(h http.Handler) *ParamMux {
	m.noParam = h
	return m
}

func (m *ParamMux) NoParamFunc(f http.HandlerFunc) *ParamMux {
	m.noParam = f
	return m
}

func NewParam(key interface{}) *ParamMux {
	return &ParamMux{key: key}
}

type MethodMux struct {
	mux map[string]http.Handler
}

func (m *MethodMux) Method(method string, h http.Handler) *MethodMux {
	m.mux[method] = h
	return m
}

func (m *MethodMux) MethodFunc(method string, f http.HandlerFunc) *MethodMux {
	m.mux[method] = f
	return m
}

func (m *MethodMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f := m.mux[r.Method]

	if f == nil {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	f.ServeHTTP(w, r)
}

func NewMethod() *MethodMux {
	return &MethodMux{
		mux: make(map[string]http.Handler),
	}
}

type LoggerMux struct {
	http.Handler
}

func (l LoggerMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%v", r.URL)
	l.Handler.ServeHTTP(w, r)
}

func NewLogger(h http.Handler) *LoggerMux {
	return &LoggerMux{h}
}
