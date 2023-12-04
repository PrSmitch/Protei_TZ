package main

import (
	"context"
	"encoding/json"
	"fmt"
	user "github.com/PrSmitch/Protei_TZ/proto_generated"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
)

type Config struct {
	Grpc GRPC
	Http HTTP
}

type GRPC struct {
	Ip                string `json:"ip"`
	Port              string `json:"port"`
	Queue_size        int    `json:"queue_size"`
	Handler_pool_size int    `json:"handler_pool_size"`
}

type HTTP struct {
	IP   string       `json:"ip"`
	Port string       `json:"port"`
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
}

func (s myServer) ModifyUser(context.Context, *user.UserRequest) (*user.UserResponse, error) {
	// здесь будет вся обработка нужная, пока возвращает простой пример
	return &user.UserResponse{User: &user.UserEmployee{
		Id:        1234,
		Name:      "Sergey :)",
		WorkPhone: 12332,
		Email:     "fff@mail.ru",
		DateFrom:  "ss",
		DateTo:    "sss",
	}}, nil
}

func main() {
	config := LoadConfiguration("config.json")
	address := config.Grpc.Ip + ":" + config.Grpc.Port
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("can't create listener: %s", err)
	}
	serverRegistrar := grpc.NewServer()
	service := &myServer{}
	user.RegisterUserServiceServer(serverRegistrar, service)
	err = serverRegistrar.Serve(lis)
	if err != nil {
		log.Fatalf("can't serve: %s", err)
	}
}