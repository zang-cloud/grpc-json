package grpcj

import (
	"bytes"
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/joncalhoun/qson"
	"github.com/sirupsen/logrus"
	"github.com/zang-cloud/grpc-json/jsonpb"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"time"
)

const (
	defaultPort    = ":8080"
	defaultTimeout = 30 * time.Second
)

var defaultMarshaler = jsonpb.Marshaler{EnumsAsInts: true, EmitDefaults: true, OrigName: true, Int64AsString: false, Uint64AsString: false}
var defaultUnmarshaler = jsonpb.Unmarshaler{AllowUnknownFields: false}

type serverOpts struct {
	port                string
	timeout             time.Duration
	marshaler           jsonpb.Marshaler
	unmarshaler         jsonpb.Unmarshaler
	endpointToMethodMap map[string]interface{}
	allowedMethods      []string
	middlewareHandlers  []MiddlewareFunc
	healthcheckEndpoint string
	healthcheckFunc     func() error
	healthcheckInterval time.Duration
}

func (s *serverOpts) isAllowedMethod(methodName string) bool {
	if len(s.allowedMethods) < 1 {
		return true
	}
	for _, method := range s.allowedMethods {
		if methodName == method {
			return true
		}
	}
	return false
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

// AddEndpoints allows adding endpoints that are mapped to GRPC methods. It takes a map of URL path to GRPC method.
// The URL path must include the starting / (e.g. "/MyAddedEndpoint").
func AddEndpoints(endpointToMethodMap map[string]interface{}) func(*serverOpts) {
	return func(s *serverOpts) {
		s.endpointToMethodMap = endpointToMethodMap
	}
}

// AllowedMethods allows restricting access to only the defined methods.
// Pass in a slice of methods (e.g. AllowedMethods([]interface{}{server.Add})).
func AllowedMethods(allowedMethods []interface{}) func(*serverOpts) {
	return func(s *serverOpts) {
		for _, method := range allowedMethods {
			methodName := runtime.FuncForPC(reflect.ValueOf(method).Pointer()).Name()
			s.allowedMethods = append(s.allowedMethods, methodName)
		}
	}
}

var healthcheckStatus int = http.StatusOK

// HealthCheck allows defining an endpoint for healthchecks as well as a function to be executed at defined intervals to check the health of the service.
// The healthcheck function will be run at the defined intervals and will respond to http requests with 200 or 500 depending on the status of the healthcheck.
// Ideally this function should check any external dependencies such as pinging mysql etc. and should return any error.
// The endpoint name must include the starting / (e.g. "/MyHealtchCheck").
func HealthCheck(endpoint string, healthcheckFunc func() error, healthcheckInterval time.Duration) func(*serverOpts) {
	return func(s *serverOpts) {
		s.healthcheckEndpoint = endpoint
		s.healthcheckFunc = healthcheckFunc
		s.healthcheckInterval = healthcheckInterval
	}
}

// Middleware registers a middleware handler. Any number of middleware handlers can be passed in and they will be called in order.
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
	mux := http.NewServeMux()

	for i := 0; i < grpcServerType.NumMethod(); i++ {
		methodName := grpcServerType.Method(i).Name
		if httpServerOpts.isAllowedMethod(methodName) {
			methodFunc := reflect.ValueOf(grpcServer).MethodByName(methodName)
			handler := grpcjHandler(methodFunc, httpServerOpts)
			mux.HandleFunc("/"+methodName, applyMiddlewareTo(handler, httpServerOpts.middlewareHandlers).ServeHTTP)
		}
	}

	for endpoint, method := range httpServerOpts.endpointToMethodMap {
		methodName := runtime.FuncForPC(reflect.ValueOf(method).Pointer()).Name()
		if httpServerOpts.isAllowedMethod(methodName) {
			methodFunc := reflect.ValueOf(method)
			handler := grpcjHandler(methodFunc, httpServerOpts)
			mux.HandleFunc(endpoint, applyMiddlewareTo(handler, httpServerOpts.middlewareHandlers).ServeHTTP)
		}
	}

	if httpServerOpts.healthcheckFunc != nil {
		go func() {
			for _ = range time.Tick(httpServerOpts.healthcheckInterval) {
				if err := httpServerOpts.healthcheckFunc(); err != nil {
					logrus.Errorln("Healthcheck failed:", err)
					healthcheckStatus = http.StatusInternalServerError
				} else {
					if healthcheckStatus != http.StatusOK {
						logrus.Infoln("Healthcheck recovered")
					}
					healthcheckStatus = http.StatusOK
				}
			}
		}()
		mux.HandleFunc(httpServerOpts.healthcheckEndpoint, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(healthcheckStatus)
		})
	}

	serverHTTP := &http.Server{Addr: httpServerOpts.port, Handler: mux}

	// Graceful shutdown.
	idleConnsClosed := make(chan struct{})
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, os.Interrupt, os.Kill)
	go func() {
		exitSignal := <-exitChan
		fmt.Printf("Received shutdown signal '%s', attempting graceful shutdown of grpc-json server\n", exitSignal)
		if err := serverHTTP.Shutdown(context.Background()); err != nil {
			fmt.Println("Error gracefully shutting down grpc-json server:", err)
		}
		close(idleConnsClosed)

		// We need to re-emit the exit signal because the normal use case is that
		// grpc-json will be run in a goroutine and since it has hijacked the exit signal it must re-emit.
		fmt.Println("Graceful shutdown of grpc-json complete, re-emitting exit signal", exitSignal)
		signal.Stop(exitChan)
		if currentProcess, err := os.FindProcess(os.Getpid()); err != nil {
			fmt.Println("Error getting current process to re-emit exit signal:", err)
		} else {
			currentProcess.Signal(exitSignal)
		}
	}()

	if err := serverHTTP.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Println("Error listening and serving grpc-json:", err)
	}
	<-idleConnsClosed
}

func grpcjHandler(methodFunc reflect.Value, httpServerOpts *serverOpts) http.HandlerFunc {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), httpServerOpts.timeout)
		defer cancel()

		structType := methodFunc.Type().In(1).Elem()
		structInstance, _ := reflect.New(structType).Interface().(proto.Message)

		switch r.Method {
		case "POST":
			defer r.Body.Close()
			if err := httpServerOpts.unmarshaler.Unmarshal(r.Body, structInstance); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "GET":
			parsedJSON, err := qson.ToJSON(r.URL.RawQuery)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := httpServerOpts.unmarshaler.Unmarshal(ioutil.NopCloser(bytes.NewReader(parsedJSON)), structInstance); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		default:
			w.WriteHeader(http.StatusNotImplemented)
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
	return handler
}
