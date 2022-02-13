package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RomanMelnyk113/scheduler/pkg/food/v1"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

type ServiceChecker struct {
	order   food.OrderServiceClient
	kitchen food.KitchenServiceClient
	drone   food.DroneServiceClient
}

func NewServiceChecker(connection grpc.ClientConnInterface) *ServiceChecker {
	o := food.NewOrderServiceClient(connection)
	k := food.NewKitchenServiceClient(connection)
	d := food.NewDroneServiceClient(connection)
	return &ServiceChecker{o, k, d}
}

func (s *ServiceChecker) GetOrdersList(status food.Order_Status, pageSize int32, pageToken string) []*food.Order {
	request := &food.ListOrdersRequest{
		StatusFilter: status,
		PageSize:     pageSize,
		PageToken:    pageToken,
	}
	response, err := s.order.ListOrders(context.Background(), request)

	if err != nil {
		grpclog.Fatalf("Fail to load list of orders: %v", err)
	}
	return response.Orders
}

func (s *ServiceChecker) UpdateOrder(orderName string, status food.Order_Status) {
	request := &food.UpdateOrderRequest{
		Order: &food.Order{
			Name:   orderName,
			Status: status,
		},
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"status"},
		},
	}
	_, err := s.order.UpdateOrder(context.Background(), request)

	if err != nil {
		grpclog.Fatalf("fail to update order %s. Error: %v", orderName, err)
	}
}

func (s *ServiceChecker) CheckKitchenOrder(orderNames string, orderChan chan<- string, kitchenChan chan<- string) {
	names := strings.Split(orderNames, "|")
	orderName := names[0]
	kitchenOrderName := names[1]
	r1 := &food.GetKitchenOrderRequest{
		Name: kitchenOrderName,
	}
	order, err := s.kitchen.GetKitchenOrder(context.Background(), r1)
	if err != nil {
		grpclog.Fatalf("Order not found %s. Details: %v", orderNames, err)
	}
	fmt.Printf("Order from kitchen: %q\n", order)
	switch order.Status {
	case food.KitchenOrder_PACKAGED:
		// move order to the delivery dron
		shipment := s.CreateShipment()
		s.UpdateOrder(orderName, food.Order_IN_FLIGHT)
		mappedOrders := orderName + "|" + shipment.Name
		kitchenChan <- mappedOrders
	case food.KitchenOrder_NEW, food.KitchenOrder_PREPARATION:
		// sleep for same time as retry action loop and then push NEW orders back to the channel
		// so we can iterate over it again to check status
		time.Sleep(time.Second * RETRY_TIME)
		orderChan <- orderNames
	}
}

func (s *ServiceChecker) CheckDronOrder(orderNames string, kitchenChan chan<- string) {
	names := strings.Split(orderNames, "|")
	orderName := names[0]
	shipmentName := names[1]
	r1 := &food.GetShipmentRequest{
		Name: shipmentName,
	}
	shipment, err := s.drone.GetShipment(context.Background(), r1)
	if err != nil {
		grpclog.Fatalf("Shipment not found %s. Details: %v", orderNames, err)
	}
	fmt.Printf("Shipment from drone: %q\n", shipment)
	switch shipment.Status {
	case food.Shipment_DELIVERED:
		s.UpdateOrder(orderName, food.Order_DELIVERED)
	case food.Shipment_REJECTED:
		s.UpdateOrder(orderName, food.Order_REJECTED)
	case food.Shipment_NEW, food.Shipment_COLLECTED:
		// sleep for same time as retry action loop and then push NEW orders back to the channel
		// so we can iterate over it again to check status
		time.Sleep(time.Second * RETRY_TIME)
		kitchenChan <- orderNames
	}
}

func (s *ServiceChecker) SendOrderToKitchen(orderName string, orderChan chan<- string) {
	request := &food.CreateKitchenOrderRequest{
		Kitchenorder: &food.KitchenOrder{},
	}
	kitchenOrder, err := s.kitchen.CreateKitchenOrder(context.Background(), request)

	if err != nil {
		grpclog.Fatalf("Order %s sending is failed!\n", orderName)
	}

	s.UpdateOrder(orderName, food.Order_PREPARATION)
	// TODO: add error handling

	// connect orders by names
	mappedOrders := orderName + "|" + kitchenOrder.Name
	fmt.Printf("Sent kitchen: %s\n", orderName)
	orderChan <- mappedOrders
}

func (s *ServiceChecker) CreateShipment() *food.Shipment {
	r2 := &food.CreateShipmentRequest{
		Shipment: &food.Shipment{},
	}
	shipment, err := s.drone.CreateShipment(context.Background(), r2)
	if err != nil {
		grpclog.Fatalf("fail to dial: %v", err)
	}

	return shipment
}
