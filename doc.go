/*
Package grpcj starts an HTTP server and serves GRPC methods as JSON.

This package uses reflection to discover all your RPC methods and automatically unmarshal the JSON requests and pass them to the proper RPC method.
POSTing to http://mydomain/MyRPCMethodName will call the corresponding RPC method.

For example, if you have the following proto definition:
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
and the following golang implementation:
    type server struct{}

    func (server *server) Add(ctx context.Context, req *pb.AddRequest) (*pb.AddResponse, error) {
        resp := &pb.AddResponse{
            Sum: req.NumOne + req.NumTwo,
        }
        return resp, nil
    }

    grpcj.Serve(&server{})
simply POST a JSON payload of '{"num_one": 1, "num_two": 1}' to localhost:8080/Add and you will receive a response of '{"sum": 2}'.

Quickstart:

    go grpcj.Serve(&myGRPCServer{})
*/
package grpcj // import "github.com/zang-cloud/grpc-json"
