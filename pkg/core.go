package mongo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
	"os"
	"time"
)

func Install(logger *zap.Logger, config *ConfigEntity) *mongo.Database {
	logPrefix := "install mongo"
	logger.Info(fmt.Sprintf("%s %s", logPrefix, "start ->"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var clientOptions options.ClientOptions

	if config.Account != "" && config.Password != "" {
		clientOptions.SetAuth(options.Credential{
			Username: config.Account,
			Password: config.Password,
		})
	}
	if config.Tls.CaCert != "" && config.Tls.ClientCert != "" && config.Tls.ClientCertKey != "" {
		certPool := x509.NewCertPool()
		CAFile, CAErr := os.ReadFile(config.Tls.CaCert)
		if CAErr != nil {
			logger.Error(fmt.Sprintf("%s read %s error: %s", logPrefix, config.Tls.CaCert, CAErr.Error()))
			return nil
		}
		certPool.AppendCertsFromPEM(CAFile)

		clientCert, clientCertErr := tls.LoadX509KeyPair(config.Tls.ClientCert, config.Tls.ClientCertKey)
		if clientCertErr != nil {
			logger.Error(fmt.Sprintf("%s tls.LoadX509KeyPair err: %v", logPrefix, clientCertErr))
			return nil
		}

		tlsConfig := tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      certPool,
		}
		clientOptions.SetTLSConfig(&tlsConfig)
	}

	uri := fmt.Sprintf("mongodb://%s", config.Address)
	if config.Mode { // cluster
	} else {
	} // stand alone
	clientOptions.ApplyURI(uri)

	clientOptions.SetBSONOptions(&options.BSONOptions{
		UseLocalTimeZone: true,
	})

	clientOptions.SetMaxConnecting(uint64(config.MaxOpenConnects))
	clientOptions.SetMaxPoolSize(uint64(config.MaxIdleConnects))
	clientOptions.SetMaxConnIdleTime(time.Second * time.Duration(config.ConnMaxLifeTime))

	if config.LoggerEnable {
		clientOptions.Monitor = &event.CommandMonitor{
			Started: func(ctx context.Context, event *event.CommandStartedEvent) {
				logger.Info(fmt.Sprintf("[MongoDB][RequestID:%d][database:%s] %s", event.RequestID, event.DatabaseName, event.Command))
			},
			Succeeded: func(ctx context.Context, event *event.CommandSucceededEvent) {
				logger.Info(fmt.Sprintf("[MongoDB][RequestID:%d] [%s] %s", event.RequestID, event.Duration.String(), event.Reply))
			},
			Failed: func(ctx context.Context, event *event.CommandFailedEvent) {
				logger.Error(fmt.Sprintf("[MongoDB][RequestID:%d] [%s] %s", event.RequestID, event.Duration.String(), event.Failure))
			},
		}
	}

	client, cErr := mongo.Connect(ctx, &clientOptions)
	if cErr != nil {
		logger.Error(fmt.Sprintf("%s mongo client connect: %v", logPrefix, cErr))
		return nil
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		logger.Error(fmt.Sprintf("%s mongo client ping: %v", logPrefix, err))
		return nil
	}

	db := client.Database(config.Database)

	logger.Info(fmt.Sprintf("%s %s", logPrefix, "success ->"))

	return db
}
