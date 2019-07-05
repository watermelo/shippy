package main

import (
	// 导如 protoc 自动生成的包
	"context"
	// "google.golang.org/grpc"
	"log"
	// "net"
	"sync"
	"fmt"
	"github.com/micro/go-micro"
	pb "shippy/consignment-service/proto/consignment"
	vesselProto "shippy/vessel-service/proto/vessel"
)

const (
	PORT = ":50051"
)

//
// 仓库接口
//
type repository interface {
	Create(consignment *pb.Consignment) (*pb.Consignment, error) // 存放新货物
	GetAll() []*pb.Consignment
}

//
// 我们存放多批货物的仓库，实现了 repository 接口
//
type Repository struct {
	mu           sync.RWMutex
	consignments []*pb.Consignment
}

func (repo *Repository) Create(consignment *pb.Consignment) (*pb.Consignment, error) {
	repo.mu.Lock()
	defer func(){
		repo.mu.Unlock()
	}()
	repo.consignments = append(repo.consignments, consignment)
	return consignment, nil
}

func (repo *Repository) GetAll() []*pb.Consignment {
	return repo.consignments
}

//
// 定义微服务
//

type service struct {
	repo Repository
	vesselClient vesselProto.VesselServiceClient
}

//
// service 实现 consignment.pb.go 中的 ShippingServiceServer 接口
// 使 service 作为 gRPC 的服务端
//
// 托运新的货物
// func (s *service) CreateConsignment(ctx context.Context, req *pb.Consignment) (*pb.Response, error) {
func (s *service) CreateConsignment(ctx context.Context, req *pb.Consignment, resp *pb.Response) error {
	// Here we call a client instance of our vessel service with our consignment weight,
	// and the amount of containers as the capacity value
	vesselResponse, err := s.vesselClient.FindAvailable(context.Background(), &vesselProto.Specification{
		MaxWeight: req.Weight,
		Capacity: int32(len(req.Containers)),
	})
	if err != nil {
		log.Printf("Vessel error: %s \n", err)
		return err
	}
	log.Printf("Found vessel: %s \n", vesselResponse.Vessel.Name)
	// We set the VesselId as the vessel we got back from our
	// vessel service
	req.VesselId = vesselResponse.Vessel.Id

	// 接收承运的货物
	consignment, err := s.repo.Create(req)
	fmt.Println(req, "fick...")
	if err != nil {
		return err
	}
	resp.Created = true
	resp.Consignment = consignment
	return nil
}

// func (s *service) GetConsignments(ctx context.Context, req *pb.GetRequest) (*pb.Response, error) {
func (s *service) GetConsignments(ctx context.Context, req *pb.GetRequest, resp *pb.Response) error {
	// 接收承运的货物
	allConsignments := s.repo.GetAll()
	resp.Consignments = allConsignments
	return nil
}

func main() {
	/* grpc
	listener, err := net.Listen("tcp", PORT)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("listen on: %s\n", PORT)

	server := grpc.NewServer()
	repo := Repository{}

	// 向 rRPC 服务器注册微服务
	// 此时会把我们自己实现的微服务 service 与协议中的 ShippingServiceServer 绑定
	pb.RegisterShippingServiceServer(server, &service{repo})

	if err := server.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
	*/
	server := micro.NewService(
		// 必须和 consignment.proto 中的 package 一致
		micro.Name("go.micro.srv.consignment"),
		micro.Version("latest"),
	)

	// 解析命令行参数
	server.Init()
	repo := Repository{}
	vesselClient := vesselProto.NewVesselServiceClient("go.micro.srv.vessel", server.Client())

	pb.RegisterShippingServiceHandler(server.Server(), &service{repo, vesselClient})

	if err := server.Run(); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
