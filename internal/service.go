package internal

import (
	"log"

	"github.com/Mantelijo/grpc-file-stream/internal/api"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func RunService() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("could not create a new logger: %v\n", err)
	}
	l.Info("starting up the service")

	loadEnvVars()

	// Start HTTP REST API

	// Start the GRPC server
	address := viper.GetString("BIND_ADDR") + ":" + viper.GetString("BIND_PORT")
	l.Error(
		"grpc server shutdown",
		zap.Error(api.StartGRPCServer(address, l)),
	)
}

func loadEnvVars() {
	viper.AutomaticEnv()
	// Address on which gRPC server will be listening
	viper.SetDefault("BIND_ADDR", "0.0.0.0")
	// Port on which gRPC server will be exposed
	viper.SetDefault("BIND_PORT", "8888")
}
