generate_grpc_code:
             protoc --go_out=proto_generated \
           --go_opt=paths=source_relative \
           --go-grpc_out=proto_generated \
           --go-grpc_opt=paths=source_relative user.proto