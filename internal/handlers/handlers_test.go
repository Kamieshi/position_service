package handlers

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Kamieshi/position_service/internal/model"
	prst "github.com/Kamieshi/position_service/internal/priceStorage"
	"github.com/Kamieshi/position_service/internal/repository"
	"github.com/Kamieshi/position_service/protoc"
	priceProtoc "github.com/Kamieshi/price_service/protoc"
	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	positionManagerUser protoc.PositionsManagerClient
	clFromService       *protoc.GetUserResponse
	priceStorage        *prst.PriceStore
	UserServiceClient   protoc.UsersManagerClient
	UserRep             *repository.UserRepository
	companyID1          = "8550780b-f246-4355-bb52-eba180b00896"
	companies           = []string{
		"8550780b-f246-4355-bb52-eba180b00896",
	}
)

func TestMain(m *testing.M) {
	PositionService, err := grpc.Dial("localhost:5302", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	connPriceService, err := grpc.Dial("localhost:5300", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	pool, err := pgxpool.Connect(context.Background(), "postgres://postgres:postgres@localhost:5432/postgres")
	if err != nil {
		log.WithError(err).Fatal()
	}

	UserRep = &repository.UserRepository{Pool: pool}

	priceServiceClient := priceProtoc.NewOwnPriceStreamClient(connPriceService)
	priceStorage = prst.NewPriceStore(context.Background(), priceServiceClient)
	go priceStorage.ListenStream(context.Background())
	UserServiceClient = protoc.NewUsersManagerClient(PositionService)
	positionManagerUser = protoc.NewPositionsManagerClient(PositionService)
	time.Sleep(1 * time.Second)
	clProtoc := protoc.User{
		Name:    "TestUser",
		Balance: 1000000,
	}
	_, err = UserServiceClient.CreateUser(context.Background(), &protoc.CreateUserRequest{
		User: &clProtoc,
	})
	if err != nil {
		log.Info(clProtoc, " is exist")
	}
	clFromService, err = UserServiceClient.GetUser(context.Background(), &protoc.GetUserRequest{Name: clProtoc.Name})
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(1 * time.Second)

	code := m.Run()
	os.Exit(code)
}

func TestUnFixedBuyPosition(t *testing.T) {
	price, err := priceStorage.GetPrice(companyID1)
	if err != nil {
		t.Fatal(err)
	}
	openPositionRequest := &protoc.OpenPositionRequest{
		Price: &protoc.Price{
			Company: &protoc.Company{
				ID:   companyID1,
				Name: "Company name",
			},
			Ask:  price.Ask,
			Bid:  price.Bid,
			Time: price.Time.Format("2006-01-02T15:04:05.000TZ-07:00"),
		},
		UserID:           clFromService.User.ID,
		CountBuyPosition: 1,
		IsFixed:          false,
		IsSales:          false,
	}
	t.Log("StartOpen")
	res, err := positionManagerUser.OpenPosition(context.Background(), openPositionRequest)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(res)
	time.Sleep(1 * time.Second)
	price, err = priceStorage.GetPrice(companyID1)
	if err != nil {
		t.Fatal(err)
	}
	closePositionRequest := &protoc.ClosePositionRequest{
		PositionID: res.ID,
		Price: &protoc.Price{
			Company: &protoc.Company{
				ID:   companyID1,
				Name: "Company name",
			},
			Ask:  price.Ask,
			Bid:  price.Bid,
			Time: price.Time.Format("2006-01-02T15:04:05.000TZ-07:00"),
		},
		UserID: clFromService.User.ID,
	}
	respClosePosition, err := positionManagerUser.ClosePosition(context.Background(), closePositionRequest)
	if err != nil {
		t.Error(err)
	}
	expectedProfit := int64(price.Bid*openPositionRequest.CountBuyPosition) - int64(openPositionRequest.Price.Ask*openPositionRequest.CountBuyPosition)
	t.Log(expectedProfit)
	if expectedProfit != respClosePosition.Profit {
		t.Error("Not equal", expectedProfit, respClosePosition.Profit)
	}
}

func TestUnFixedSalePosition(t *testing.T) {
	price, err := priceStorage.GetPrice(companyID1)
	if err != nil {
		t.Fatal(err)
	}
	openPositionRequest := &protoc.OpenPositionRequest{
		Price: &protoc.Price{
			Company: &protoc.Company{
				ID:   companyID1,
				Name: "Company name",
			},
			Ask:  price.Ask,
			Bid:  price.Bid,
			Time: price.Time.Format("2006-01-02T15:04:05.000TZ-07:00"),
		},
		UserID:           clFromService.User.ID,
		CountBuyPosition: 1,
		IsFixed:          false,
		IsSales:          true,
	}
	res, err := positionManagerUser.OpenPosition(context.Background(), openPositionRequest)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	price, err = priceStorage.GetPrice(companyID1)
	if err != nil {
		t.Fatal(err)
	}

	closePositionRequest := &protoc.ClosePositionRequest{
		PositionID: res.ID,
		Price: &protoc.Price{
			Company: &protoc.Company{
				ID:   companyID1,
				Name: "Company name",
			},
			Ask:  price.Ask,
			Bid:  price.Bid,
			Time: price.Time.Format("2006-01-02T15:04:05.000TZ-07:00"),
		},
		UserID: clFromService.User.ID,
	}
	respClosePosition, err := positionManagerUser.ClosePosition(context.Background(), closePositionRequest)
	if err != nil {
		t.Error(err)
	}
	dopProfit := int64(openPositionRequest.CountBuyPosition * openPositionRequest.Price.Bid)
	expectedProfit := int64(price.Bid*openPositionRequest.CountBuyPosition) - int64(openPositionRequest.Price.Bid*openPositionRequest.CountBuyPosition) + dopProfit
	t.Log(expectedProfit)
	if expectedProfit != respClosePosition.Profit {
		t.Error("Not equal", expectedProfit, respClosePosition.Profit)
	}

}

func TestFixedBuyPosition(t *testing.T) {
	price, err := priceStorage.GetPrice(companyID1)
	if err != nil {
		t.Fatal(err)
	}
	openPositionRequest := &protoc.OpenPositionRequest{
		Price: &protoc.Price{
			Company: &protoc.Company{
				ID:   companyID1,
				Name: "Company name",
			},
			Ask:  price.Ask,
			Bid:  price.Bid,
			Time: price.Time.Format("2006-01-02T15:04:05.000TZ-07:00"),
		},
		UserID:           clFromService.User.ID,
		CountBuyPosition: 1,
		MaxProfit:        1,
		MinProfit:        -100,
		IsFixed:          true,
		IsSales:          false,
	}
	startAsk := price.Ask
	res, err := positionManagerUser.OpenPosition(context.Background(), openPositionRequest)
	if err != nil {
		t.Fatal(err)
	}
	for {
		price, err = priceStorage.GetPrice(companyID1)
		if err != nil {
			t.Fatal(err)
		}
		if price.Bid-startAsk > 15 {
			break
		}
	}
	pool, err := pgxpool.Connect(context.Background(), "postgres://postgres:postgres@localhost:5432/postgres")
	if err != nil {
		log.WithError(err).Fatal()
	}
	var r bool
	time.Sleep(5 * time.Second)
	err = pool.QueryRow(context.Background(), "SELECT is_opened FROM positions WHERE id=$1", res.ID).Scan(&r)
	if err != nil {
		t.Fatal(err)
	}
	if r == true {
		t.Fatal("Not close ", res.ID)
	}
}

func TestAddBalance(t *testing.T) {

	req := &protoc.AddBalanceRequest{
		UserID:           clFromService.User.ID,
		DifferentBalance: 1000,
	}
	_, err := UserServiceClient.AddBalance(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
}

func TestManyUsersOpenPosition(t *testing.T) {
	countUsers := 10
	countPosition := 1

	users := make([]*model.User, 0, countUsers)
	for i := 0; i < countUsers; i++ {
		users = append(users, &model.User{
			Name:    fmt.Sprintf("User N%d", i),
			Balance: 1000000,
		})
	}

	for _, user := range users {
		err := UserRep.Insert(context.Background(), user)
		if err != nil {
			userDB, err := UserRep.GetByName(context.Background(), user.Name)
			user.ID = userDB.ID
			user.Balance = userDB.Balance
			if err != nil {
				t.Fatal("Trouble with database")
			}
		}
	}
	reqResp := make(map[*protoc.OpenPositionRequest]*protoc.OpenPositionResponse)
	for _, user := range users {
		for i := 0; i < countPosition; i++ {
			for _, companyID := range companies {

				price, err := priceStorage.GetPrice(companyID)
				if err != nil {
					t.Fatal(err)
				}
				openPositionRequest := &protoc.OpenPositionRequest{
					Price: &protoc.Price{
						Company: &protoc.Company{
							ID:   companyID,
							Name: "Company name",
						},
						Ask:  price.Ask,
						Bid:  price.Bid,
						Time: price.Time.Format("2006-01-02T15:04:05.000TZ-07:00"),
					},
					UserID:           user.ID.String(),
					CountBuyPosition: 1,
					IsFixed:          false,
					IsSales:          false,
				}
				res, err := positionManagerUser.OpenPosition(context.Background(), openPositionRequest)
				if err != nil {
					t.Error(err, " ", time.Since(price.Time))
					i--
					continue
				}
				reqResp[openPositionRequest] = res
			}
		}
	}
	time.Sleep(2 * time.Second)
	for req, res := range reqResp {
		t_resp := time.Now()
		price, err := priceStorage.GetPrice(companyID1)
		if err != nil {
			t.Fatal(err)
		}
		closePositionRequest := &protoc.ClosePositionRequest{
			PositionID: res.ID,
			Price: &protoc.Price{
				Company: &protoc.Company{
					ID:   companyID1,
					Name: "Company name",
				},
				Ask:  price.Ask,
				Bid:  price.Bid,
				Time: price.Time.Format("2006-01-02T15:04:05.000TZ-07:00"),
			},
			UserID: req.UserID,
		}

		respClosePosition, err := positionManagerUser.ClosePosition(context.Background(), closePositionRequest)
		if err != nil {
			t.Error(err)
		}
		expectedProfit := int64(price.Bid*req.CountBuyPosition) - int64(req.Price.Ask*req.CountBuyPosition)
		if expectedProfit != respClosePosition.Profit {
			t.Error("Not equal", expectedProfit, respClosePosition.Profit,
				"Time before get priceStorage -> resp : ", time.Since(t_resp),
				"Time priceStorage delay : ", time.Since(price.Time))
		}
	}
}
