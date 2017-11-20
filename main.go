package grpcj

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"time"
)

const (
	defaultPort    = ":8080"
	defaultTimeout = 30 * time.Second
)

type serverOpts struct {
	port               string
	timeout            time.Duration
	middlewareHandlers []middleware
}

type middleware func(http.HandlerFunc) http.HandlerFunc

func applyMiddlewareTo(handler http.HandlerFunc, middlewareHandlers []middleware) http.HandlerFunc {
	reverse(middlewareHandlers)
	next := handler
	for _, middlewareHandler := range middlewareHandlers {
		next = middlewareHandler(next)
	}
	return next
}

func reverse(s []middleware) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Port sets the HTTP server port. Default is ":8080".
func Port(port string) func(*serverOpts) {
	return func(s *serverOpts) {
		s.port = port
	}
}

// Timeout sets the HTTP request timeout. Default is 30 seconds.
func Timeout(timeout time.Duration) func(*serverOpts) {
	return func(s *serverOpts) {
		s.timeout = timeout
	}
}

// Middleware registers a middleware handler. Any number of middleware handlers can be passed in and they will be called in order.
// A middleware handler must have a signature of func(http.HandlerFunc) http.HandlerFunc.
//
// An example middleware handler:
//		func Logger(next http.HandlerFunc) http.handlerFunc {
// 			return func(w http.ResponseWriter, r *http.Request) {
// 				fmt.Println("Got Request")
// 				next.ServeHTTP(w, r)
// 			}
// 		}
//
// To abort a request, middleware should simply
// not call the passed-in HandlerFunc:
//		func Auth(next http.HandlerFunc) http.handlerFunc {
// 			return func(w http.ResponseWriter, r *http.Request) {
// 				if isAuthorized {
// 					next.ServeHTTP(w, r)
// 				} else {
// 					fmt.Println("User is not authorized")
// 				}
// 			}
// 		}
func Middleware(handlers ...middleware) func(*serverOpts) {
	return func(s *serverOpts) {
		s.middlewareHandlers = append(s.middlewareHandlers, handlers...)
	}
}

func applyOptions(options []func(*serverOpts)) *serverOpts {
	httpServerOpts := &serverOpts{
		port:               defaultPort,
		timeout:            defaultTimeout,
		middlewareHandlers: []middleware{},
	}
	for _, opt := range options {
		opt(httpServerOpts)
	}
	return httpServerOpts
}

// Serve will start an HTTP server and serve the RPC methods.
func Serve(grpcServer interface{}, options ...func(*serverOpts)) {
	httpServerOpts := applyOptions(options)
	grpcServerType := reflect.TypeOf(grpcServer)

	for i := 0; i < grpcServerType.NumMethod(); i++ {
		methodName := grpcServerType.Method(i).Name
		methodFunc := reflect.ValueOf(grpcServer).MethodByName(methodName)

		handler := func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(context.Background(), httpServerOpts.timeout)
			defer cancel()

			structType := methodFunc.Type().In(1).Elem()
			structInstance := reflect.New(structType).Interface()

			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(structInstance); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}

			methodArgs := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(structInstance)}
			methodReturnVals := methodFunc.Call(methodArgs)

			// If we got back an error then return it
			err, _ := methodReturnVals[1].Interface().(error)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

			w.Header().Set("Content-Type", "application/json")
			resp := methodReturnVals[0].Interface()
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("An error has occured"))
				return
			}
		}

		http.HandleFunc("/"+methodName, applyMiddlewareTo(handler, httpServerOpts.middlewareHandlers))
	}

	http.ListenAndServe(httpServerOpts.port, nil)
}
