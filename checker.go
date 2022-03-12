package main

import (
	"context"
	"fmt"

	"github.com/RomanMelnyk113/scheduler/pkg/food/v1"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc"
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
		fmt.Errorf("Fail to load list of orders: %v\n", err)
		return nil
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
		fmt.Errorf("fail to update order %s. Error: %v\n", orderName, err)
	}
}

func (s *ServiceChecker) CheckKitchenOrder(orderName string) {
	n, ok := kitchenOrders.Load(orderName)
	if !ok {
		fmt.Errorf("Order not found in kitchen list: %s\n", orderName)
		return
	}
	kitchenOrderName := n.(string)
	r1 := &food.GetKitchenOrderRequest{
		Name: kitchenOrderName,
	}
	order, err := s.kitchen.GetKitchenOrder(context.Background(), r1)
	if err != nil {
		fmt.Errorf("Order not found %s. Details: %v\n", kitchenOrderName, err)
		return
	}
	switch order.Status {
	case food.KitchenOrder_PACKAGED:
		// move order to the delivery dron
		shipment := s.CreateShipment()
		droneOrders.Store(orderName, shipment.Name)
		s.UpdateOrder(orderName, food.Order_IN_FLIGHT)

		// remove order from the kitchen list
		kitchenOrders.Delete(kitchenOrderName)
		fmt.Printf("Order %s has been sent to delivery dron\n", orderName)
	}
}

func (s *ServiceChecker) CheckDronOrder(orderName string) {
	n, ok := droneOrders.Load(orderName)
	if !ok {
		fmt.Errorf("Order not found in drone list: %s\n", orderName)
		return
	}
	shipmentName := n.(string)
	r1 := &food.GetShipmentRequest{
		Name: shipmentName,
	}
	shipment, err := s.drone.GetShipment(context.Background(), r1)
	if err != nil {
		fmt.Errorf("Shipment not found %s. Details: %v\n", shipmentName, err)
	}

	var status food.Order_Status
	switch shipment.Status {
	case food.Shipment_DELIVERED:
		status = food.Order_DELIVERED
		fmt.Printf("Order %s has been successfully develivered.\n", orderName)
	case food.Shipment_REJECTED:
		status = food.Order_REJECTED
		fmt.Printf("Order %s has been rejected by customer.\n", orderName)
	}

	if status == food.Order_DELIVERED || status == food.Order_REJECTED {
		s.UpdateOrder(orderName, status)
		droneOrders.Delete(orderName)
	}
}

func (s *ServiceChecker) SendOrderToKitchen(orderName string) {
	request := &food.CreateKitchenOrderRequest{
		Kitchenorder: &food.KitchenOrder{},
	}
	kitchenOrder, err := s.kitchen.CreateKitchenOrder(context.Background(), request)

	if err != nil {
		fmt.Errorf("Order %s sending is failed!\n", orderName)
		return
	}

	s.UpdateOrder(orderName, food.Order_PREPARATION)
	// TODO: add error handling

	// connect orders by names
	kitchenOrders.Store(orderName, kitchenOrder.Name)
	fmt.Printf("Order %s has been sent to kitchen.\n", orderName)
}

func (s *ServiceChecker) CreateShipment() *food.Shipment {
	r2 := &food.CreateShipmentRequest{
		Shipment: &food.Shipment{},
	}
	shipment, err := s.drone.CreateShipment(context.Background(), r2)
	if err != nil {
		fmt.Errorf("Failed to create shipment: %v\n", err)
		return nil
	}
	return shipment
}
