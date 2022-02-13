package main

import (
	"crypto/tls"
	"fmt"
	"time"

	food "github.com/RomanMelnyk113/scheduler/pkg/food/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

const RETRY_TIME = 20

type Scheduler struct {
	checker ServiceChecker
}

func NewScheduler() *Scheduler {
	return new(Scheduler)
}

func (s *Scheduler) connect(address string) *grpc.ClientConn {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	if err != nil {
		grpclog.Fatalf("fail to dial: %v", err)
	}

	return conn
}

func (s *Scheduler) run(address string) {
	conn := s.connect(address)
	defer conn.Close()

	s.checker = *NewServiceChecker(conn)

	// order channel to handle list of new orders mapped to kitchen orders
	// Example: orders/04c9325b-56c9-4c70-abd8-5af165e0827a|kitchenorders/04c9325b-56c9-4c70-abd8-5af165e0827b
	orderChan := make(chan string)

	// similar to orderChan but to keep tracking kitchen orders which should be sent to dron delivery
	kitchenChan := make(chan string)

	fmt.Println("Start listening...")
	go s.listenKitchenOrders(orderChan, kitchenChan)
	go s.listenDronOrders(kitchenChan)
	for {
		go s.checkOrders(orderChan)
		time.Sleep(time.Second * RETRY_TIME)
	}
}

func (s *Scheduler) checkOrders(orderChan chan string) {
	fmt.Println("Checking for a new orders...")
	orders := s.checker.GetOrdersList(food.Order_NEW, 100, "")
	orders_count := len(orders)
	if orders_count == 0 {
		fmt.Printf("No new orders found. Retry in %d seconds\n", RETRY_TIME)
		return
	}
	fmt.Printf("Found %d new orders, sending to kitchen.", orders_count)

	for _, order := range orders {
		go s.checker.SendOrderToKitchen(order.Name, orderChan)
	}
}

// keep tracking new orders channel for ready to sending to kitchen orders
func (s *Scheduler) listenKitchenOrders(orderChan chan string, kitchenChan chan string) {
	for {
		select {
		case v := <-orderChan:
			go s.checker.CheckKitchenOrder(v, orderChan, kitchenChan)
		case <-time.After(time.Millisecond):
		}
	}
}

// keep traking kitchen channel for ready to shipment orders
func (s *Scheduler) listenDronOrders(kitchenChan chan string) {
	for {
		select {
		case v := <-kitchenChan:
			go s.checker.CheckDronOrder(v, kitchenChan)
		case <-time.After(time.Millisecond):
		}
	}
}

func main() {
	scheduler := NewScheduler()
	scheduler.run("127.0.0.1:9000")
}
