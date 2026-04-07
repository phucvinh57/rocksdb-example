package mongodb

import (
	"context"
	"fmt"
	"os"
	"time"
	"trading-bsx/pkg/config"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Order *mongo.Collection
var Raw *mongo.Database

func Init() {
	if !config.MongoEnabled() {
		Order = nil
		Raw = nil
		log.Info().Msg("MongoDB disabled")
		return
	}

	mongodbUri := os.Getenv("MONGODB_URI")
	if len(mongodbUri) == 0 {
		panic("MONGODB_URI is required")
	}

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(mongodbUri).SetServerAPIOptions(serverAPI)

	var err error
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}

	var result bson.M
	if err := client.Database("admin").RunCommand(context.TODO(), bson.M{"ping": 1}).Decode(&result); err != nil {
		panic(err)
	}

	dbName := "bsx-trading"
	if os.Getenv("ENV") == "test" {
		dbName = fmt.Sprintf("test-%d-bsx-trading", time.Now().UnixMilli())
	}

	Raw = client.Database(dbName)
	Order = Raw.Collection("orders")

	bgCtx := context.Background()
	Order.Indexes().CreateOne(bgCtx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}},
	})

	log.Info().Msg("MongoDB connected")
}
