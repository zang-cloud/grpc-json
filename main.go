package grpcjson

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
	port    string
	timeout time.Duration
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

func applyOptions(options []func(*serverOpts)) *serverOpts {
	httpServerOpts := &serverOpts{
		port:    defaultPort,
		timeout: defaultTimeout,
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

		http.HandleFunc("/"+methodName, func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			ctx, cancel := context.WithTimeout(context.Background(), httpServerOpts.timeout)
			defer cancel()

			structType := methodFunc.Type().In(1).Elem()
			structInstance := reflect.New(structType).Interface()

			if err := json.NewDecoder(r.Body).Decode(structInstance); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
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
		})
	}

	go http.ListenAndServe(httpServerOpts.port, nil)
}
