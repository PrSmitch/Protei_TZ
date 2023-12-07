package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	user "github.com/PrSmitch/Protei_TZ/proto_generated"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
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
		return nil, status.Errorf(codes.ResourceExhausted, "Очередь полная")
	}
}

func (s *myServer) processRequests(req *user.ModifyUserRequest) (*user.ModifyUserResponse, error) {
	for _, val := range req.Users {
		reqEmployeeJSON, _ := json.Marshal(val.Employee)
		reqEmployee, err := http.NewRequest("POST", "http://localhost:8080/process", bytes.NewBuffer(reqEmployeeJSON))
		if err != nil {
			log.Fatalf("Ошибка при СОЗДАНИИ запроса: %s", err)
		}
		reqEmployee.Header.Add("Authorization", s.basicAuth)
		reqEmployee.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respEmployee, err := client.Do(reqEmployee)
		if err != nil {
			log.Fatalf("Ошибка при ОТПРАВКЕ запроса: %s", err)
		}
		defer respEmployee.Body.Close()
		bodyEmployee, _ := io.ReadAll(respEmployee.Body)
		EmployeeResponse := new(EmployeeContract)
		err = json.Unmarshal(bodyEmployee, EmployeeResponse)
		if err != nil {
			log.Fatalf("Ошибка при UNMARSHAL запроса: %s", err)
		}

		val.Absence.Id = []int64{EmployeeResponse.Id}
		reqAbsenceJSON, _ := json.Marshal(val.Absence)
		reqAbsence, err := http.NewRequest("POST", "http://localhost:8080/process", bytes.NewBuffer(reqAbsenceJSON))
		if err != nil {
			log.Fatalf("Ошибка при СОЗДАНИИ запроса: %s", err)
		}
		reqAbsence.Header.Add("Authorization", s.basicAuth)
		reqAbsence.Header.Set("Content-Type", "application/json")
		respAbsence, err := client.Do(reqAbsence)
		if err != nil {
			log.Fatalf("Ошибка при ОТПРАВКЕ запроса: %s", err)
		}
		defer respAbsence.Body.Close()
		bodyAbsence, _ := io.ReadAll(respAbsence.Body)

		AbsenceResponse := new(AbsenceContract)
		err = json.Unmarshal(bodyAbsence, AbsenceResponse)
		if err != nil {
			log.Fatalf("Ошибка при UNMARSHAL запроса: %s", err)
		}
	}
	ans := &user.ModifyUserResponse{}
	ans.Users = make([]*user.UserInfo, len(req.Users))
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

type EmployeeContract struct {
	Id          int64  `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	WorkPhone   int64  `json:"workPhone"`
}

type AbsenceContract struct {
	Id         int64  `json:"id"`
	PersonID   int64  `json:"personid"`
	CreateDate string `json:"createDate"`
	DateFrom   string `json:"dateFrom"`
	ReasonID   int64  `json:"reasonid"`
}

func main() {
	config := LoadConfiguration("config.json")

	username := config.Http.Auth.Username
	password := config.Http.Auth.Password
	auth := username + ":" + password
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	fmt.Println("Запускается GRPC-сервер")

	address := config.Grpc.Ip + ":" + config.Grpc.Port
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Ошибка при создании listner: %s", err)
	}
	serverRegistrar := grpc.NewServer()
	service := newServer(config.Grpc.QueueSize, config.Grpc.HandlerPoolSize, basicAuth)
	user.RegisterUserServiceServer(serverRegistrar, service)
	go func() {
		err = serverRegistrar.Serve(lis)
		if err != nil {
			log.Fatalf("Ошибка при serve: %s", err)
		}
	}()
	server := newServer(config.Grpc.QueueSize, config.Grpc.HandlerPoolSize, basicAuth) // 100 - размер очереди, 10 - количество обработчиков.
	server.startWorkers()
	time.Sleep(60 * time.Second)
	serverRegistrar.GracefulStop()
}
