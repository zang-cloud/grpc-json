package grpcj

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/zang-cloud/grpc-json/jsonpb"
	"net/http"
	"reflect"
	"time"
)

const (
	defaultPort    = ":8080"
	defaultTimeout = 30 * time.Second
)

var defaultMarshaler = jsonpb.Marshaler{EnumsAsInts: true, EmitDefaults: true, OrigName: true, Int64AsString: false, Uint64AsString: false}
var defaultUnmarshaler = jsonpb.Unmarshaler{AllowUnknownFields: false}

type serverOpts struct {
	port               string
	timeout            time.Duration
	marshaler          jsonpb.Marshaler
	unmarshaler        jsonpb.Unmarshaler
	middlewareHandlers []MiddlewareFunc
}

// The MiddlewareFunc type is for use in the Middlware option
type MiddlewareFunc func(http.Handler) http.Handler

func applyMiddlewareTo(handler http.Handler, middlewareHandlers []MiddlewareFunc) http.Handler {
	next := handler
	for _, middlewareHandler := range middlewareHandlers {
		next = middlewareHandler(next)
	}
	return next
}

func reverse(s []MiddlewareFunc) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Port allows setting the HTTP server port. Default is ":8080".
func Port(port string) func(*serverOpts) {
	return func(s *serverOpts) {
		s.port = port
	}
}

// Timeout allows setting the HTTP request timeout. Default is 30 seconds.
func Timeout(timeout time.Duration) func(*serverOpts) {
	return func(s *serverOpts) {
		s.timeout = timeout
	}
}

// Marshaler allows defining the JSON marshaler. Default marshaler is the github.com/zang-cloud/grpc-json/jsonpb.go Marshaler{EnumsAsInts: true, EmitDefaults: true, OrigName: true, Int64AsString: false, Uint64AsString: false}.
// The Marshaler is a copy of the github.com/golang/protobuf/jsonpb/jsonpb.go Marshaler but adds 2 options: Int64AsString and Uint64AsString.
// These options were added to allow returning Int64 and Uint64 as numbers instead of strings.
func Marshaler(marshaler jsonpb.Marshaler) func(*serverOpts) {
	return func(s *serverOpts) {
		s.marshaler = marshaler
	}
}

// Unmarshaler allows defining the JSON unmarshaler. Default unmarshaler is the github.com/zang-cloud/grpc-json/jsonpb.go Unmarshaler{AllowUnknownFields: false}.
// The Unmarshaler is a copy of the github.com/golang/protobuf/jsonpb/jsonpb.go Unmarshaler.
func Unmarshaler(unmarshaler jsonpb.Unmarshaler) func(*serverOpts) {
	return func(s *serverOpts) {
		s.unmarshaler = unmarshaler
	}
}

// The Middleware option registers a middleware handler. Any number of middleware handlers can be passed in and they will be called in order.
// A middleware handler must have a signature of func(http.Handler) http.Handler.
//
// An example middleware handler:
//		func Logger(next http.Handler) http.Handler {
// 			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 				fmt.Println("Got Request")
// 				next.ServeHTTP(w, r)
// 			})
// 		}
//
// To abort a request, middleware should simply not call the passed-in Handler:
//		func Auth(next http.Handler) http.Handler {
// 			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 				if isAuthorized {
// 					next.ServeHTTP(w, r)
// 				} else {
// 					fmt.Println("User is not authorized")
// 				}
// 			})
// 		}
//
// Because the middleware signature is the same as github.com/gorilla/handlers, those middleware handlers can be used as well.
// For example, to use the gorilla CORS middleware:
//		grpcj.Serve(&grpcServer{}, grpcj.Middleware(handlers.CORS()))
func Middleware(handlers ...MiddlewareFunc) func(*serverOpts) {
	return func(s *serverOpts) {
		s.middlewareHandlers = append(s.middlewareHandlers, handlers...)
	}
}

// BasicAuth is a MiddlewareFunc that enforces basic auth.
func BasicAuth(username, password string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)

			authUsername, authPassword, ok := r.BasicAuth()
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if authUsername != username || authPassword != password {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func applyOptions(options []func(*serverOpts)) *serverOpts {
	httpServerOpts := &serverOpts{
		port:               defaultPort,
		timeout:            defaultTimeout,
		marshaler:          defaultMarshaler,
		unmarshaler:        defaultUnmarshaler,
		middlewareHandlers: []MiddlewareFunc{},
	}
	for _, opt := range options {
		opt(httpServerOpts)
	}
	return httpServerOpts
}

// Serve will start an HTTP server and serve the RPC methods.
func Serve(grpcServer interface{}, options ...func(*serverOpts)) {
	httpServerOpts := applyOptions(options)
	reverse(httpServerOpts.middlewareHandlers)
	grpcServerType := reflect.TypeOf(grpcServer)

	for i := 0; i < grpcServerType.NumMethod(); i++ {
		methodName := grpcServerType.Method(i).Name
		methodFunc := reflect.ValueOf(grpcServer).MethodByName(methodName)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(context.Background(), httpServerOpts.timeout)
			defer cancel()

			structType := methodFunc.Type().In(1).Elem()
			structInstance, _ := reflect.New(structType).Interface().(proto.Message)

			defer r.Body.Close()
			if err := httpServerOpts.unmarshaler.Unmarshal(r.Body, structInstance); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			methodArgs := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(structInstance)}
			methodReturnVals := methodFunc.Call(methodArgs)

			// If we got back an error then return it
			err, _ := methodReturnVals[1].Interface().(error)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			resp, _ := methodReturnVals[0].Interface().(proto.Message)
			if err := httpServerOpts.marshaler.Marshal(w, resp); err != nil {
				http.Error(w, "An error has occured", http.StatusInternalServerError)
				return
			}
		})

		http.HandleFunc("/"+methodName, applyMiddlewareTo(handler, httpServerOpts.middlewareHandlers).ServeHTTP)
	}

	http.ListenAndServe(httpServerOpts.port, nil)
}
