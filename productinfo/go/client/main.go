package main

import (
	"context"
	"contrib.go.opencensus.io/exporter/jaeger"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"go.opencensus.io/examples/exporter"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/resolver"
	"io/ioutil"
	"log"
	"path/filepath"
	pb "productinfo/client/ecommerce"
	"time"
)

const (
	exampleScheme      = "example"
	exampleServiceName = "lb.example.grpc.io"
	hostname           = "localhost"
	address            = "localhost:5000"
	enableTls          = true
)

var (
	addrs = []string{"localhost:50051", "localhost:50052"}
	//crtFile = filepath.Join("productinfo", "certs", "server.crt") //one way
	crtFile = filepath.Join("productinfo", "certs", "client.crt")
	keyFile = filepath.Join("productinfo", "certs", "client.key")
	caFile  = filepath.Join("productinfo", "certs", "ca.crt")
)

func main() {
	var (
		transportCred grpc.DialOption
	)

	if enableTls {
		// --- Using tls --
		//creds, err := credentials.NewClientTLSFromFile(crtFile, hostname)
		//if err != nil {
		//	log.Fatalf("failed to load credentials: %v", err)
		//}
		//transportCred = grpc.WithTransportCredentials(creds)

		// --- Using the OS CA --
		//certPool := x509.SystemCertPool()
		//creds := credentials.NewClientTLSFromCert(certPool, hostname)
		//transportCred = grpc.WithTransportCredentials(creds)

		// --- Using mTls --
		// Load the client certificates from disk
		certificate, err := tls.LoadX509KeyPair(crtFile, keyFile)
		if err != nil {
			log.Fatalf("could not load client key pair: %s", err)
		}

		// Create a certificate pool from the certificate authority
		certPool := x509.NewCertPool()
		ca, err := ioutil.ReadFile(caFile)
		if err != nil {
			log.Fatalf("could not read ca certificate: %s", err)
		}

		// Append the certificates from the CA
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			log.Fatalf("failed to append ca certs")
		}

		// transport credentials.
		transportCred = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			ServerName:   hostname,
			Certificates: []tls.Certificate{certificate},
			RootCAs:      certPool,
		}))

	} else {
		transportCred = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	//auth := &basicAuth{
	//	username: "admin",
	//	password: "admin",
	//}

	auth := oauth.NewOauthAccess(fetchToken())

	//auth, err := oauth.NewJWTAccessFromFile("token.json")
	//if err != nil {
	//	log.Fatal("Failed to create JWT credentials:", err)
	//}

	//auth, err := oauth.NewServiceAccountFromFile("service-account.json")
	//if err != nil {
	//	log.Fatal("Failed to create JWT credentials:", err)
	//}

	initTracing()

	// Register stats and trace exporters to export
	// the collected data.
	view.RegisterExporter(&exporter.PrintExporter{})
	// Register the view to collect gRPC client stats.
	if err := view.Register(ocgrpc.DefaultClientViews...); err != nil {
		log.Fatal(err)
	}

	opts := []grpc.DialOption{
		transportCred,
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
		grpc.WithPerRPCCredentials(auth),
		grpc.WithUnaryInterceptor(
			grpc_middleware.ChainUnaryClient(
				orderUnaryClientInterceptor,
			),
		),
		grpc.WithStreamInterceptor(
			grpc_middleware.ChainStreamClient(
				streamInterceptor,
			),
		),
		//grpc.WithUnaryInterceptor(orderUnaryClientInterceptor),
		//grpc.WithStreamInterceptor(streamInterceptor),
	}

	traditionalServiceCall(opts)
}

func initTracing() {
	// This is a demo app with low QPS. trace.AlwaysSample() is used here
	// to make sure traces are available for observation and analysis.
	// In a production environment or high QPS setup please use
	// trace.ProbabilitySampler set at the desired probability.
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	agentEndpointURI := "localhost:6831"
	collectorEndpointURI := "http://localhost:14268/api/traces"
	exporter, err := jaeger.NewExporter(jaeger.Options{
		CollectorEndpoint: collectorEndpointURI,
		AgentEndpoint:     agentEndpointURI,
		Process: jaeger.Process{
			ServiceName: "product_info",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)

}

func traditionalServiceCall(opts []grpc.DialOption) {
	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// =========== UNARY ===========
	ctx, span := trace.StartSpan(ctx, "ecommerce.ProductInfoClient")
	productInfo(ctx, conn, span)
	span.End()

	// =========== STREAMING ===========
	orderTest(conn, ctx, cancel)
}

// Load Balancing
func thickClient(opts []grpc.DialOption) {
	firstConn, err := grpc.Dial(
		fmt.Sprintf("%s:///%s", exampleScheme, exampleServiceName),
		opts...,
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	defer func(conn *grpc.ClientConn) {
		if err := conn.Close(); err != nil {
			log.Fatal("Error closing connection", err)
		}
	}(firstConn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	log.Println("==== Calling ecommerce.ProductInfo/AddProduct with pick_first ====")
	productInfo(ctx, firstConn, nil)

	roundRobinConn, err := grpc.Dial(
		fmt.Sprintf("%s:///%s", exampleScheme, exampleServiceName),
		grpc.WithDefaultServiceConfig(
			fmt.Sprintf(`{"loadBalancingConfig": [{"%s":{}}]}`, roundrobin.Name),
		),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	defer func(conn *grpc.ClientConn) {
		if err := conn.Close(); err != nil {
			log.Fatal("Error closing connection", err)
		}
	}(roundRobinConn)

	log.Println("==== Calling ecommerce.ProductInfo/AddProduct with round_robin ====")
	productInfo(ctx, roundRobinConn, nil)
}

func productInfo(ctx context.Context, conn *grpc.ClientConn, span *trace.Span) {
	compressor := grpc.UseCompressor(gzip.Name)
	c := pb.NewProductInfoClient(conn)
	name := "Apple iPhone 13"
	description := `Meet Apple iPhone 13. All-new triple-camera system with Ultra Wide and Night mode`
	price := int32(100_000)

	id, err := c.AddProduct(
		ctx,
		&pb.Product{
			Name:        name,
			Description: description,
			Price:       price,
		},
		compressor,
	)
	if err != nil {
		if span != nil {
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: err.Error()})
		}
		log.Fatalf("Couldn't add product: %v", err)
	}

	log.Printf("Product ID: %s added successfully", id.Value)

	product, err := c.GetProduct(ctx, &pb.ProductID{Value: id.Value}, compressor)
	if err != nil {
		if span != nil {
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: err.Error()})
		}
		log.Fatalf("Couldn't get product: %v", err)
	}

	log.Print("Product:", product.String())
}

// ---------------------------- OAuth ----------------------------

func fetchToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken: "some-secret-token",
	}
}

// ---------------------------- Basic Auth ----------------------------
type basicAuth struct {
	username string
	password string
}

func (b *basicAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	auth := b.username + ":" + b.password
	enc := base64.StdEncoding.EncodeToString([]byte(auth))
	return map[string]string{
		"authorization": "Basic " + enc,
	}, nil
}

func (b *basicAuth) RequireTransportSecurity() bool {
	return true
}

// ---------------------------- Interceptor ----------------------------
type wrappedStream struct {
	grpc.ClientStream
}

func (w *wrappedStream) RecvMsg(m interface{}) error {
	log.Printf("====== [Client Stream Interceptor] "+
		"Receive a message (Type: %T) at %v", m, time.Now().Format(time.RFC3339))
	return w.ClientStream.RecvMsg(m)
}

func (w *wrappedStream) SendMsg(m interface{}) error {
	log.Printf("====== [Client Stream Interceptor] "+
		"Send a message (Type: %T) at %v", m, time.Now().Format(time.RFC3339))

	return w.ClientStream.SendMsg(m)
}

func streamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	log.Println("======== [Client Interceptor] ", method)

	s, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		return nil, err
	}
	return &wrappedStream{s}, nil
}

func orderUnaryClientInterceptor(ctx context.Context, method string, req interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	log.Println("Method:", method)

	err := invoker(ctx, method, req, reply, cc, opts...)

	log.Println(reply)
	return err
}

// ---------------------------- Name Resolver ----------------------------
type exampleResolver struct {
	target     resolver.Target
	cc         resolver.ClientConn
	addrsStore map[string][]string
}

func (r *exampleResolver) start() {
	//addrStrs := r.addrsStore[r.target.Endpoint]
	addrStrs := r.addrsStore[r.target.URL.Path]
	addrs := make([]resolver.Address, len(addrStrs))
	for i, s := range addrStrs {
		addrs[i] = resolver.Address{Addr: s}
	}
	_ = r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (*exampleResolver) ResolveNow(_ resolver.ResolveNowOptions) {}
func (*exampleResolver) Close()                                  {}

type exampleResolverBuilder struct{}

func (*exampleResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {

	r := &exampleResolver{
		target: target,
		cc:     cc,
		addrsStore: map[string][]string{
			exampleServiceName: addrs,
		},
	}

	r.start()
	return r, nil
}

func (*exampleResolverBuilder) Scheme() string { return exampleScheme }

func init() {
	resolver.Register(&exampleResolverBuilder{})
}
