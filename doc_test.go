package grpcj

import "time"

type grpcServer struct{}

func ExamplePort() {
	Serve(&grpcServer{}, Port(":8080"))
}

func ExampleTimeout() {
	Serve(&grpcServer{}, Timeout(5*time.Second))
}

func ExampleServe() {
	// With no options set, will default to port :8080 and request timeout of 30 seconds.
	Serve(&grpcServer{})
}
