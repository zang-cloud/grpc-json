`grpcjson` starts an HTTP server in the background and serves GRPC methods as JSON.

Quickstart
----------
`grpcjson.Serve(&myGRPCServer{}, grpcjson.Port(":8080"), grpcjson.Timeout(30*time.Second))`

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
then simple POST a JSON payload of `{"num_one": 1, "num_two": 2}` to `http://mydomain/Add` and you will receive a response of `{"sum": 2}`.
