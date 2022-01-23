package main

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/wrappers"
	epb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"io"
	"log"
	pb "productinfo/client/ecommerce"
	"time"
)

func orderTest(conn *grpc.ClientConn, ctx context.Context, cancel context.CancelFunc) {
	orderMgtClient := pb.NewOrderManagementClient(conn)
	retrievedOrder, err := orderMgtClient.GetOrder(ctx, &wrappers.StringValue{Value: "106"})
	log.Print("GetOrder Response -> : ", retrievedOrder)

	// =========== SERVER STREAMING ===========
	searchStream, _ := orderMgtClient.SearchOrders(ctx, &wrapperspb.StringValue{Value: "Google"})

	for {
		searchOrder, err := searchStream.Recv()
		if err == io.EOF {
			break
		}
		log.Print("Search Result: ", searchOrder)
	}

	// =========== CLIENT STREAM ===========
	updateStream, err := orderMgtClient.UpdateOrders(ctx)
	if err != nil {
		log.Fatalf("%v.UpdateOrders(_) = _, %v", orderMgtClient, err)
	}

	updOrder1 := pb.Order{Id: "102", Items: []string{"Google Pixel 3A", "Google Pixel Book"}, Destination: "Mountain View, CA", Price: 110_000}
	updOrder2 := pb.Order{Id: "103", Items: []string{"Apple Watch S4", "Mac Book Pro", "iPad Pro"}, Destination: "San Jose, CA", Price: 280_000}
	updOrder3 := pb.Order{Id: "104", Items: []string{"Google Home Mini", "Google Nest Hub", "iPad Mini"}, Destination: "Mountain View, CA", Price: 220_000}

	if err := updateStream.Send(&updOrder1); err != nil {
		log.Fatalf("%v.Send(%v) = %v", updateStream, &updOrder1, err)
	}
	if err := updateStream.Send(&updOrder2); err != nil {
		log.Fatalf("%v.Send(%v) = %v", updateStream, &updOrder2, err)
	}
	if err := updateStream.Send(&updOrder3); err != nil {
		log.Fatalf("%v.Send(%v) = %v", updateStream, &updOrder3, err)
	}
	//var header, trailer metadata.MD

	header, err := updateStream.Header()
	if err == nil {
		log.Print("Streaming Header:", header)
	}
	trailer := updateStream.Trailer()
	log.Print("Streaming Trailer:", trailer)

	updateRes, err := updateStream.CloseAndRecv()
	if err != nil {
		log.Fatalf("%v.CloseAndRecv() got error %v, want %v", updateStream, err, nil)
	}
	log.Printf("Update Orders Res: %s", updateRes)

	// =========== BIDIRECTIONAL STREAM ===========
	processOrders(ctx, orderMgtClient, cancel)

	metaData := metadata.Pairs(
		"timestamp", time.Now().Format(time.StampNano),
		"kn", "vn",
	)

	// RPC using the context with new metadata.
	metaDataCtx := metadata.NewOutgoingContext(context.Background(), metaData)
	ctxA := metadata.AppendToOutgoingContext(metaDataCtx,
		"k1", "v1",
		"k1", "v2",
		"k2", "v3",
	)

	order1 := pb.Order{
		Id:          "101", // "-1"
		Items:       []string{"iPhone XS", "Mac Book Pro"},
		Destination: "San Jose, CA",
		Price:       230_000,
	}

	res, addOrderError := orderMgtClient.AddOrder(ctxA, &order1, grpc.Header(&header), grpc.Trailer(&trailer))
	if addOrderError != nil {
		errorCode := status.Code(addOrderError)
		if errorCode == codes.InvalidArgument {
			log.Printf("Invalid Argument Error: %s", errorCode)
			errorStatus := status.Convert(addOrderError)

			for _, d := range errorStatus.Details() {
				switch info := d.(type) {
				case *epb.BadRequest_FieldViolation:
					log.Println("Request Field Invalid:", info)
				default:
					log.Println("Unexpected error type:", info)
				}
			}
		} else {
			log.Println("Unhandled error:", errorCode)
		}
	} else {
		log.Println("AddOrder Response ->", res.Value)
	}

	if t, ok := header["timestamp"]; ok {
		log.Printf("timestamp from header:\n")
		for i, e := range t {
			fmt.Printf(" %d. %s\n", i, e)
		}
	} else {
		log.Fatal("timestamp expected but doesn't exist in header")
	}
	if l, ok := header["location"]; ok {
		log.Printf("location from header:\n")
		for i, e := range l {
			fmt.Printf(" %d. %s\n", i, e)
		}
	} else {
		log.Fatal("location expected but doesn't exist in header")
	}
}

func processOrders(ctx context.Context, orderMgtClient pb.OrderManagementClient, cancel context.CancelFunc) {
	streamProcOrder, _ := orderMgtClient.ProcessOrders(ctx)

	if err := streamProcOrder.Send(&wrapperspb.StringValue{Value: "102"}); err != nil {
		log.Fatalf("%v.Send(%v) = %v", orderMgtClient, "102", err)
	}
	if err := streamProcOrder.Send(&wrapperspb.StringValue{Value: "103"}); err != nil {
		log.Fatalf("%v.Send(%v) = %v", orderMgtClient, "103", err)
	}
	if err := streamProcOrder.Send(&wrapperspb.StringValue{Value: "104"}); err != nil {
		log.Fatalf("%v.Send(%v) = %v", orderMgtClient, "104", err)
	}

	channel := make(chan bool, 1)
	go asyncClientBidirRPC(streamProcOrder, channel)
	time.Sleep(time.Millisecond * 1000)

	//cancel()
	log.Printf(" RPC Status : %s", ctx.Err())

	if err := streamProcOrder.Send(&wrapperspb.StringValue{Value: "106"}); err != nil {
		log.Fatalf("%v.Send(%v) = %v", orderMgtClient, "106", err)
	}
	if err := streamProcOrder.CloseSend(); err != nil {
		log.Fatal(err)
	}

	<-channel
}

func asyncClientBidirRPC(streamProcOrder pb.OrderManagement_ProcessOrdersClient, c chan bool) {
	for {
		combinedShipment, errProcOrder := streamProcOrder.Recv()
		if errProcOrder == io.EOF {
			break
		}
		if errProcOrder != nil {
			log.Printf("Error Receiving messages %v", errProcOrder)
			break
		}
		log.Print("Combined shipment: ", combinedShipment.OrdersList)
	}
	c <- true
}
