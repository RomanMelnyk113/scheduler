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
	//orderClient   clients.OrderClient
	//kitchenClient clients.KitchenClient
	//droneClient   clients.DroneClient
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
	//s.orderClient = *clients.NewOrderClient(conn)
	//s.kitchenClient = *clients.NewKitchenClient(conn)
	//s.droneClient = *clients.NewDroneClient(conn)

	//r1 := &food.GetKitchenOrderRequest{
	//Name: "kitchenorders/02b972ed-9597-43f2-a420-6930819e038a",
	//}
	//order, err := s.kitchenClient.GetKitchenOrder(context.Background(), r1)
	//if err != nil {
	//grpclog.Fatalf("fail to dial: %v", err)
	//}
	//fmt.Println("Order from kitchen:")
	//fmt.Print(order)

	orderChan := make(chan string)
	kitchenChan := make(chan string)
	//channels := map[string]chan string{
	//"order":   make(chan string),
	//"kitchen": make(chan string),
	//"dron":    make(chan string),
	//}
	//c3 := make(chan string)
	fmt.Println("Start listening...")
	go s.listenKitchenOrders(orderChan, kitchenChan)
	go s.listenDronOrders(kitchenChan)
	for {
		go s.checkOrders(orderChan)
		//go s.checkKitchen(c1, c2)
		//go s.listenDronDelivery(c)
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

//func (s *Scheduler) checkKitchen(c1 chan string, c2 chan string) {
func (s *Scheduler) listenKitchenOrders(orderChan chan string, kitchenChan chan string) {
	for {
		select {
		case v := <-orderChan:
			go s.checker.CheckKitchenOrder(v, orderChan, kitchenChan)
		case <-time.After(time.Millisecond):
		}
	}
}

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
