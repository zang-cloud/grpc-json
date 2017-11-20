package grpcj

import (
	"fmt"
	"net/http"
	"time"
)

type grpcServer struct{}

func ExamplePort() {
	Serve(&grpcServer{}, Port(":8080"))
}

func ExampleTimeout() {
	Serve(&grpcServer{}, Timeout(5*time.Second))
}

func ExampleMiddleware() {
	loggerOne := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("First Middleware")
			next.ServeHTTP(w, r)
		}
	}

	loggerTwo := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("Second Middleware")
			next.ServeHTTP(w, r)
		}
	}

	Serve(&grpcServer{}, Middleware(loggerOne, loggerTwo))
}

func ExampleServe() {
	// With no options set, will default to port :8080 and request timeout of 30 seconds.
	Serve(&grpcServer{})
}
