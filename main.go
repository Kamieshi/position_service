package main

import (
	"context"
	"net"
	"runtime"

	"github.com/Kamieshi/position_service/internal/config"
	"github.com/Kamieshi/position_service/internal/handlers"
	"github.com/Kamieshi/position_service/internal/priceStorage"
	"github.com/Kamieshi/position_service/internal/repository"
	"github.com/Kamieshi/position_service/internal/service"
	"github.com/Kamieshi/position_service/internal/userStorage"
	"github.com/Kamieshi/position_service/protoc"
	priceProtoc "github.com/Kamieshi/price_service/protoc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	log.SetLevel(log.DebugLevel)
	conf, err := config.GetConfig()
	if err != nil {
		log.WithError(err).Fatal()
	}
	listener, err := net.Listen("tcp", ":"+conf.PositionServicePortRPC)
	if err != nil {
		log.WithError(err).Fatal()
	}
	pool, err := pgxpool.Connect(context.Background(), conf.GetConnStringPostgres())
	if err != nil {
		log.WithError(err).Fatal()
	}

	connToPriceService, err := grpc.Dial(conf.GetConnStringToPriceService(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.WithError(err).Fatal()
	}

	positionRep := &repository.PositionRepository{Pool: pool}
	userRep := &repository.UserRepository{Pool: pool}
	clientPriceService := priceProtoc.NewOwnPriceStreamClient(connToPriceService)

	userService := userStorage.NewUserService(context.Background(), userRep)

	prStore := priceStorage.NewPriceStore(context.Background(), clientPriceService)
	go prStore.ListenStream(context.Background())

	PositionService := &service.PositionsService{
		UsersPositions:     make(map[uuid.UUID]*service.UserPositions),
		PriceStorage:       prStore,
		UserStorage:        userService,
		PositionRepository: positionRep,
		CtxApp:             context.Background(),
	}
	go PositionService.SyncPositionService(context.Background())
	grpcServer := grpc.NewServer()
	protoc.RegisterPositionsManagerServer(grpcServer, &handlers.PositionManagerServerImplement{PositionsManager: PositionService})
	protoc.RegisterUsersManagerServer(grpcServer, &handlers.UsersManagerServerImplement{UserService: userService})
	log.Info("gRPC ActivePosition service_old server start")
	log.Info("Count core : ", runtime.NumCPU())
	log.Info("Addr Listen: ", conf.GetConnStringToPositionService())
	log.Info(grpcServer.Serve(listener))
	log.Info("gRPC ActivePosition service_old  server Stop")
}
