package grpcj

import (
	"fmt"
	"github.com/gorilla/handlers"
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
	logger := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("First Middleware")
			next.ServeHTTP(w, r)
		})
	}

	// Middleware handlers from github.com/gorilla/handlers can be used as well.

	Serve(&grpcServer{}, Middleware(logger, handlers.CORS()))
}

func ExampleBasicAuth() {
	Serve(&grpcServer{}, Middleware(BasicAuth("my_username", "my_password")))
}

func ExampleServe() {
	// With no options set, will default to port :8080 and request timeout of 30 seconds.
	Serve(&grpcServer{})
}
