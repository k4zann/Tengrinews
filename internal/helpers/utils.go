package helpers

import (
	"context"
	"log"
	"regexp"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CutImageData(content string) (string, string) {
	re := regexp.MustCompile(`\{.*?"@context".*?\}`)
	matchContent := re.FindString(content)
	cleanedContent := re.ReplaceAllString(content, "")
	return cleanedContent, matchContent
}

func InitMongoDB() (*mongo.Client, *mongo.Collection, error) {
	clientOptions := options.Client().ApplyURI(MongoDBUri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, nil, err
	}

	err = client.Ping(context.Background(), nil)
	if err != nil {
		return nil, nil, err
	}

	collection := client.Database("news").Collection("articles")
	log.Println("Connected to MongoDB successfully")
	return client, collection, nil
}

// func LoadEnv() {
// 	err := godotenv.Load(".env")
// 	if err != nil {
// 		log.Fatal("Error loading .env file")
// 	}
// }
