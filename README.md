[![GoDoc](https://godoc.org/github.com/zang-cloud/grpc-json?status.svg)](https://godoc.org/github.com/zang-cloud/grpc-json)

`grpc-json` starts an HTTP server and serves GRPC methods as JSON.

Quickstart
----------
This will start an HTTP server on port 8080:

`go grpcj.Serve(&myGRPCServer{})`

Options can be set like this:

`go grpcj.Serve(&myGRPCServer{}, grpcj.Port(":8080"), grpcj.Timeout(30*time.Second))`

Overview
--------
This package uses reflection to discover all your RPC methods and automatically unmarshal the JSON requests and pass them to the proper RPC method.
`POST`ing to `http://mydomain/MyRPCMethodName` will call the corresponding RPC method.

For example, if you have the following proto definition:
```
service MyService {
    rpc Add(AddRequest) returns (AddResponse) {}
}

message AddRequest {
    int64 num_one = 1;
    int64 num_two = 2;
}

message AddResponse {
    int64 sum = 1;
}
```
and the following golang implementation:
```
type server struct{}

func (server *server) Add(ctx context.Context, req *pb.AddRequest) (*pb.AddResponse, error) {
    resp := &pb.AddResponse{
        Sum: req.NumOne + req.NumTwo,
    }
    return resp, nil
}

grpcj.Serve(&server{})
```
simply POST a JSON payload of `{"num_one": 1, "num_two": 1}` to localhost:8080/Add and you will receive a response of `{"sum": 2}`.

See the [godocs](https://godoc.org/github.com/zang-cloud/grpc-json) for more information.
