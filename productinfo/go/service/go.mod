module productinfo/service

go 1.17

//protoc -I ./proto ./proto/product_info.proto --go_out=plugins=grpc:./productinfo/go/service

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.1
	github.com/gofrs/uuid v4.2.0+incompatible
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	go.opencensus.io v0.23.0
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013
	google.golang.org/grpc v1.43.0
	google.golang.org/protobuf v1.27.1
)

require (
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.0 // indirect
	github.com/uber/jaeger-client-go v2.25.0+incompatible // indirect
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b // indirect
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9 // indirect
	golang.org/x/sys v0.0.0-20200930185726-fdedc70b468f // indirect
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/api v0.29.0 // indirect
)
