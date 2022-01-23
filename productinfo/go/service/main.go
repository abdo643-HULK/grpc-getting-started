package main

import (
	"context"
	"contrib.go.opencensus.io/exporter/jaeger"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	"github.com/uber/jaeger-client-go/log/zap"
	"go.opencensus.io/examples/exporter"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"go.opencensus.io/zpages"
	epb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	wrapper "google.golang.org/protobuf/types/known/wrapperspb"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path/filepath"
	pb "productinfo/service/ecommerce"
	"strings"
	"sync"
	"time"
)

type AuthType int8

const (
	Basic AuthType = iota
	OAuth
	JWT
	GoogleToken
)

const (
	port           = ":5000"
	orderBatchSize = 3
	authType       = OAuth
)

var (
	ports              = []string{":50051", ":50052"}
	crtFile            = filepath.Join("productinfo", "certs", "server.crt")
	keyFile            = filepath.Join("productinfo", "certs", "server.key")
	caFile             = filepath.Join("productinfo", "certs", "ca.crt")
	errMissingMetadata = status.Errorf(codes.InvalidArgument, "missing metadata")
	errInvalidToken    = status.Errorf(codes.Unauthenticated, "invalid credentials")
)

func main() {
	// Start z-Pages server.
	go func() {
		mux := http.NewServeMux()
		zpages.Handle(mux, "/debug")
		log.Fatal(http.ListenAndServe("127.0.0.1:8081", mux))
	}()

	startServer(port)
}

func startServerWithLB() {
	var wg sync.WaitGroup
	for _, port := range ports {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			startServer(addr)
		}(port)
	}
	wg.Wait()
}

func startServer(port string) {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	cert, err := tls.LoadX509KeyPair(crtFile, keyFile)
	if err != nil {
		log.Fatalf(" failed to load key pair: %s", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(caFile)
	if err != nil {
		log.Fatalf("could not read ca certificate: %s", err)
	}

	// Append the client certificates from the CA
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		log.Fatalf("failed to append client certs")
	}

	// initialize opencensus jaeger exporter
	initTracing()
	// Register stats and trace exporters to export
	// the collected data.
	view.RegisterExporter(&exporter.PrintExporter{})

	// Register the views to collect server request count.
	if err := view.Register(ocgrpc.DefaultServerViews...); err != nil {
		log.Fatal(err)
	}

	zapLogger := zap.Logger{}

	opts := []grpc.ServerOption{
		// Create a gRPC Server with stats handler.
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
		// Enable TLS for all incoming connections.
		grpc.Creds( // Create the TLS credentials
			credentials.NewTLS(&tls.Config{
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{cert},
				ClientCAs:    certPool,
			}),
		),
		//// single tls
		//grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				ensureValidToken,
				orderUnaryServerInterceptor,
				grpc_ctxtags.UnaryServerInterceptor(),
				grpc_recovery.UnaryServerInterceptor(),
				grpc_opentracing.UnaryServerInterceptor(),
				grpc_zap.UnaryServerInterceptor(&zapLogger),
				//grpc_auth.UnaryServerinterceptor(myAuthFunction),
			),
		),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				orderServerStreamInterceptor,
				grpc_ctxtags.StreamServerInterceptor(),
				grpc_recovery.StreamServerInterceptor(),
				grpc_opentracing.StreamServerInterceptor(),
				grpc_zap.StreamServerInterceptor(&zapLogger),
				//grpc_auth.StreamServerinterceptor(myAuthFunction),
			),
		),

		//grpc.UnaryInterceptor(ensureValidToken),
		//grpc.StreamInterceptor(orderServerStreamInterceptor),
	}

	s := grpc.NewServer(opts...)

	ser := server{
		productMap: make(map[string]*pb.Product),
		orderMap:   make(map[string]pb.Order),
		addr:       port,
	}

	initSampleData(ser.orderMap)

	pb.RegisterProductInfoServer(s, &ser)
	pb.RegisterOrderManagementServer(s, &ser)

	// Register reflection service on gRPC server.
	reflection.Register(s)

	//log.Printf("Starting gRPC listener on port " + port)
	log.Printf("Starting gRPC listener on %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to server: %v", err)
	}
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
		ServiceName:       "product_info",
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)

}

func initSampleData(orders map[string]pb.Order) {
	orders["102"] = pb.Order{Id: "102", Items: []string{"Google Pixel 3A", "Mac Book Pro"}, Destination: "Mountain View, CA", Price: 180_000}
	orders["103"] = pb.Order{Id: "103", Items: []string{"Apple Watch S4"}, Destination: "San Jose, CA", Price: 40_000}
	orders["104"] = pb.Order{Id: "104", Items: []string{"Google Home Mini", "Google Nest Hub"}, Destination: "Mountain View, CA", Price: 40_000}
	orders["105"] = pb.Order{Id: "105", Items: []string{"Amazon Echo"}, Destination: "San Jose, CA", Price: 3000}
	orders["106"] = pb.Order{Id: "106", Items: []string{"Amazon Echo", "Apple iPhone XS"}, Destination: "Mountain View, CA", Price: 30_000}
}

// valid validates the authorization.
func valid(authorization []string) bool {
	if len(authorization) < 1 {
		return false
	}

	if authType == Basic {
		token := strings.TrimPrefix(authorization[0], "Basic ")
		return token == base64.StdEncoding.EncodeToString([]byte("admin:admin"))
	} else {
		token := strings.TrimPrefix(authorization[0], "Bearer ")
		return token == "some-secret-token"
	}
}

// ensureValidToken ensures a valid token exists within a request's metadata. If
// the token is missing or invalid, the interceptor blocks execution of the
// handler and returns an error. Otherwise, the interceptor invokes the unary
// handler.
func ensureValidToken(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errMissingMetadata
	}
	// The keys within metadata.MD are normalized to lowercase.
	// See: https://godoc.org/google.golang.org/grpc/metadata#New
	if !valid(md["authorization"]) {
		return nil, errInvalidToken
	}
	// Continue execution of handler after ensuring a valid token.
	return handler(ctx, req)
}

// ----------------------- Order/Product Server ----------------------------
type server struct {
	// productMap is the field on the struct and the rest is the type: a string map that has pointers to the
	// Product struct from proto
	productMap map[string]*pb.Product
	orderMap   map[string]pb.Order
	addr       string
}

func (s *server) AddOrder(ctx context.Context, order *pb.Order) (*wrapper.StringValue, error) {
	if order.Id == "-1" {
		log.Printf("Order ID is invalid! -> Received Order ID %s", order.Id)
		errorStatus := status.New(codes.InvalidArgument, "Invalid information received")
		ds, err := errorStatus.WithDetails(
			&epb.BadRequest_FieldViolation{
				Field:       " ID",
				Description: fmt.Sprintf("Order ID received is not valid %s : %s", order.Id, order.Description),
			},
		)
		if err != nil {
			return nil, errorStatus.Err()
		}
		return nil, ds.Err()
	}

	s.orderMap[order.Id] = *order
	log.Println("Order:", order.Id, " -> Added")

	md, metadataAvailable := metadata.FromIncomingContext(ctx)
	if !metadataAvailable {
		return nil, status.Errorf(codes.DataLoss, "UnaryEcho: failed to get metadata")
	}
	if t, ok := md["timestamp"]; ok {
		fmt.Println("timestamp from metadata:")
		for i, e := range t {
			fmt.Printf("====> Metadata %d. %s\n", i, e)
		}
	}

	// Creating and sending a header.
	header := metadata.New(map[string]string{"location": "San Jose", "timestamp": time.Now().Format(time.StampNano)})
	grpc.SendHeader(ctx, header)

	trailer := metadata.Pairs(" trailer-key", "val")
	grpc.SetTrailer(ctx, trailer)

	return &wrapper.StringValue{Value: "Order Added: " + order.Id}, nil
}

func (s *server) ProcessOrders(ordersServer pb.OrderManagement_ProcessOrdersServer) error {
	batchMarker := 1
	var combinedShipmentMap = make(map[string]pb.CombinedShipment)

	md, metadataAvailable := metadata.FromIncomingContext(ordersServer.Context())
	if !metadataAvailable {
		return status.Errorf(codes.DataLoss, "UnaryEcho: failed to get metadata")
	}
	if t, ok := md["timestamp"]; ok {
		fmt.Println("timestamp from metadata:")
		for i, e := range t {
			fmt.Printf("====> Metadata %d. %s\n", i, e)
		}
	}

	// create and send header
	header := metadata.Pairs(" header-key", "val")
	ordersServer.SendHeader(header)

	// create and set trailer
	trailer := metadata.Pairs(" trailer-key", "val")
	ordersServer.SetTrailer(trailer)

	for {
		orderId, err := ordersServer.Recv()
		log.Printf("Reading Proc order : %s", orderId)
		if err == io.EOF {
			for _, comb := range combinedShipmentMap {
				if err := ordersServer.Send(&comb); err != nil {
					log.Println(err)
					return err
				}
			}
			return nil
		}

		if err != nil {
			log.Println("Err:", err)
			return err
		}

		destination := s.orderMap[orderId.GetValue()].Destination
		shipment, found := combinedShipmentMap[destination]
		log.Printf(shipment.String())

		if found {
			ord := s.orderMap[orderId.GetValue()]
			shipment.OrdersList = append(shipment.OrdersList, &ord)
			combinedShipmentMap[destination] = shipment
		} else {
			comShip := pb.CombinedShipment{Id: "cmb - " + (destination), Status: "Processed!"}
			ord := s.orderMap[orderId.GetValue()]
			comShip.OrdersList = append(shipment.OrdersList, &ord)
			combinedShipmentMap[destination] = comShip
			log.Print(len(comShip.OrdersList), comShip.GetId())
		}

		if batchMarker != orderBatchSize {
			batchMarker++
			continue
		}

		for _, comb := range combinedShipmentMap {
			if err := ordersServer.Send(&comb); err != nil {
				log.Println(err)
				return err
			}
		}
		batchMarker = 0
		combinedShipmentMap = make(map[string]pb.CombinedShipment)
	}
}

func (s *server) UpdateOrders(ordersServer pb.OrderManagement_UpdateOrdersServer) error {
	ordersStr := "Updated Order IDs: "
	for {
		order, err := ordersServer.Recv()
		if err == io.EOF {
			return ordersServer.SendAndClose(&wrapper.StringValue{Value: "Orders processed " + ordersStr})
		}

		s.orderMap[order.Id] = *order

		log.Printf("Order ID %s %s", order.Id, ": Updated")
		ordersStr += order.Id + ", "
	}
}

func (s *server) GetOrder(ctx context.Context, id *wrapper.StringValue) (*pb.Order, error) {
	order, exists := s.orderMap[id.Value]

	if exists {
		return &order, status.New(codes.OK, "").Err()
	}

	return nil, status.Errorf(codes.NotFound, "Order doesn't exist", id.Value)
}

func (s *server) SearchOrders(query *wrapper.StringValue, ordersServer pb.OrderManagement_SearchOrdersServer) error {
	for key, order := range s.orderMap {
		log.Print(key, order)
		for _, item := range order.Items {
			log.Print(item)
			if strings.Contains(item, query.Value) {
				err := ordersServer.Send(&order)
				if err != nil {
					return fmt.Errorf("error sending message to stream: %v", err)
				}
				log.Print("Matching Order Found: " + key)
				break
			}
		}
	}

	return nil
}

func (s *server) AddProduct(ctx context.Context, newProduct *pb.Product) (*pb.ProductID, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error while generating Product ID", err)
	}

	newProduct.Id = id.String()
	s.productMap[newProduct.Id] = newProduct

	return &pb.ProductID{Value: newProduct.Id}, status.New(codes.OK, "").Err()
}

func (s *server) GetProduct(ctx context.Context, id *pb.ProductID) (*pb.Product, error) {
	ctx, span := trace.StartSpan(ctx, "ecommerce.GetProduct")
	defer span.End()

	product, exists := s.productMap[id.Value]

	if exists {
		return product, status.New(codes.OK, "").Err()
	}

	return nil, status.Errorf(codes.NotFound, "Product doesn't exist", id.Value)
}

func orderUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	log.Println("======== [Server Interceptor]", info.FullMethod)

	m, err := handler(ctx, req)

	log.Printf("Post Proc Message: %s", m)
	return m, err
}

type wrappedStream struct {
	grpc.ServerStream
}

func (w *wrappedStream) RecvMsg(m interface{}) error {
	log.Printf("====== [Server Stream Interceptor Wrapper] "+
		"Receive a message (Type: %T) at %s", m, time.Now().Format(time.RFC3339))

	return w.ServerStream.RecvMsg(m)
}

func (w *wrappedStream) SendMsg(m interface{}) error {
	log.Printf("====== [Server Stream Interceptor Wrapper] "+
		"Send a message (Type: %T) at %v", m, time.Now().Format(time.RFC3339))

	return w.ServerStream.SendMsg(m)
}

func newWrappedStream(s grpc.ServerStream) grpc.ServerStream {
	return &wrappedStream{s}
}

func orderServerStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// Pre-processing
	log.Println("====== [Server Stream Interceptor] ", info.FullMethod)

	// Invoking the StreamHandler to complete the execution of RPC invocation
	err := handler(srv, newWrappedStream(ss))
	if err != nil {
		log.Printf("RPC failed with error %v", err)
	}

	return err
}
