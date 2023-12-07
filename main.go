package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	user "github.com/PrSmitch/Protei_TZ/proto_generated"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
)

type Config struct {
	Grpc GRPC
	Http HTTP
}

type GRPC struct {
	Ip              string `json:"ip"`
	Port            string `json:"port"`
	QueueSize       int    `json:"queue_size"`
	HandlerPoolSize int    `json:"handler_pool_size"`
}

type HTTP struct {
	IP   string       `json:"ip"`
	Port string       `json:"port"`
	Url  string       `json:"url"`
	Auth ExternalAuth `json:"auth"`
}

type ExternalAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoadConfiguration(file string) Config {
	var config Config
	configFile, err := os.Open(file)
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		return Config{}
	}
	return config
}

type myServer struct {
	user.UnimplementedUserServiceServer
	queue         chan *user.ModifyUserRequest
	workers       int
	processedData map[string]*[]user.UserInfo
	mutex         sync.Mutex
	basicAuth     string
}

func newServer(queueSize, workers int, basicAuth string) *myServer {
	return &myServer{
		queue:         make(chan *user.ModifyUserRequest, queueSize),
		workers:       workers,
		processedData: make(map[string]*[]user.UserInfo),
		basicAuth:     basicAuth,
	}
}

func (s *myServer) ModifyUser(ctx context.Context, req *user.ModifyUserRequest) (*user.ModifyUserResponse, error) {
	select {
	case s.queue <- req:
		return s.processRequests(req)
	default:
		return nil, grpc.Errorf(grpc.Code(resourceExhausted), "Очередь полная")
	}
	// return &user.ModifyUserResponse{Users: make([]*user.UserInfo, 2)}, nil
	/* return &user.ModifyUserResponse{ User: &user.UserEmployee{
		Id:        1234,
		Name:      "Sergey :)",
		WorkPhone: 12332,
		Email:     "fff@mail.ru",
		DateFrom:  "ss",
		DateTo:    "sss",
	}}, nil */
}

func (s *myServer) processRequests(req *user.ModifyUserRequest) (*user.ModifyUserResponse, error) {
	for key, val := range req.Users {
		reqEmployeeJSON, _ := json.Marshal(val.Employee)

		respEmployeeJSON, _ := http.Post("http://localhost:8080/Portal/springApi/api/employees", "application/json", bytes.NewBuffer(reqEmployeeJSON))
		fmt.Println(respEmployeeJSON.Body)
		// reqAbsenceJSON, _ := json.Marshal(req.Absence)
	}
	var ans *user.ModifyUserResponse
	ans.Users = req.Users
	return ans, nil
}

func (s *myServer) startWorkers() {
	for i := 0; i < s.workers; i++ {
		go s.worker()
	}
}

func (s *myServer) worker() {
	for {
		select {
		case req := <-s.queue:
			_, _ = s.processRequests(req)
		}
	}
}

func main() {
	config := LoadConfiguration("config.json")

	username := config.Http.Auth.Username
	password := config.Http.Auth.Password
	url := fmt.Sprintf("http://%s:%s/Portal/springApi/api", config.Http.IP, config.Http.Port)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Ошибка при СОЗДАНИИ запроса: %s", err)
	}
	auth := username + ":" + password
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Add("Authorization", basicAuth)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Ошибка при ОТПРАВКЕ запроса: %s", err)
	}
	defer resp.Body.Close()
	fmt.Println("Аутентификация выполнена, запускается GRPC-сервер")

	address := config.Grpc.Ip + ":" + config.Grpc.Port
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Ошибка при создании listner: %s", err)
	}
	serverRegistrar := grpc.NewServer()
	service := newServer(config.Grpc.QueueSize, config.Grpc.HandlerPoolSize, basicAuth)
	user.RegisterUserServiceServer(serverRegistrar, service)
	err = serverRegistrar.Serve(lis)
	if err != nil {
		log.Fatalf("Ошибка при serve: %s", err)
	}
}
