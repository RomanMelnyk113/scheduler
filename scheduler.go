package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"sync"
	"time"

	food "github.com/RomanMelnyk113/scheduler/pkg/food/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

//var statusNameMapping = map[food.Order_Status]string{
//food.Order_NEW: "NEW",
//food.Order_PREPARATION: "PREPARATION",
//food.Order_IN_FLIGHT: "IN_FLIGHT",
//}

// store orders sent to kitchen
var kitchenOrders = sync.Map{}

// store orders ready for delivery
var droneOrders = sync.Map{}

type Scheduler struct {
	address        string
	retry_interval time.Duration

	checker       ServiceChecker
	kitchenOrders sync.Map
	droneOrders   sync.Map
}

func NewScheduler(address string, retry_interval time.Duration) *Scheduler {
	return &Scheduler{
		address:        address,
		retry_interval: time.Duration(retry_interval.Seconds()),
	}
}

func (s *Scheduler) connect() *grpc.ClientConn {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	conn, err := grpc.Dial(s.address, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	if err != nil {
		grpclog.Fatalf("fail to dial: %v", err)
	}

	return conn
}

func (s *Scheduler) Run() {
	conn := s.connect()
	defer conn.Close()

	s.checker = *NewServiceChecker(conn)
	fmt.Println("Start listening...")
	for {
		go s.checkNewOrders()
		time.Sleep(time.Second * s.retry_interval)
		go s.checkKitchenOrders()
		go s.checkDroneOrders()
	}
}

// keep tracking new orders channel for ready to sending to kitchen orders
func (s *Scheduler) checkNewOrders() {
	orders := s.getOrdersByStatus(food.Order_NEW, 100)
	for _, order := range orders {
		go s.checker.SendOrderToKitchen(order.Name)
	}
}

// keep tracking new orders channel for ready to sending to kitchen orders
func (s *Scheduler) checkKitchenOrders() {
	orders := s.getOrdersByStatus(food.Order_PREPARATION, 100)
	for _, order := range orders {
		go s.checker.CheckKitchenOrder(order.Name)
	}
}

// keep traking kitchen channel for ready to shipment orders
func (s *Scheduler) checkDroneOrders() {
	orders := s.getOrdersByStatus(food.Order_IN_FLIGHT, 100)
	for _, order := range orders {
		go s.checker.CheckDronOrder(order.Name)
	}
}

func (s *Scheduler) getOrdersByStatus(status food.Order_Status, limit int32) []*food.Order {
	//fmt.Printf("Checking for a orders with status %d...\n", status)
	orders := s.checker.GetOrdersList(status, limit, "")
	orders_count := len(orders)
	if orders_count == 0 {
		//fmt.Printf("No new orders found. Retry in %d seconds\n", s.retry_interval)
		return nil
	}
	fmt.Printf("Found %d orders with status %d\n", orders_count, status)

	return orders
}

func main() {
	host := flag.String("host", "127.0.0.1", "gRPC host to connect")
	port := flag.String("port", "9000", "gRPC port to connect")
	retry_interval := flag.Duration("retry-interval", time.Second*60, "Time interval in seconds between checking orders.")

	flag.Parse()

	ip2 := net.ParseIP(*host)
	if ip2 == nil {
		grpclog.Fatalf("Invalid address: %s", *host)
	}
	address := *host + ":" + *port
	scheduler := NewScheduler(address, *retry_interval)
	scheduler.Run()
}
