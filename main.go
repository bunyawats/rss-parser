package main

import (
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io"
	"log"
	"net/http"
	"os"
)

type (
	Feed struct {
		Entries []Entry `xml:"entry"`
	}

	Entry struct {
		Link struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Thumbnail struct {
			URL string `xml:"url,attr"`
		} `xml:"thumbnail"`
		Title string `xml:"title"`
	}

	Request struct {
		URL string `่json:"url"`
	}
)

const (
	mongoUri              = "MONGO_URI"
	mongoDatabase         = "MONGO_DATABASE"
	collectionNameRecipes = "recipes"
	rabbitMqUri           = "RABBITMQ_URI"
	rabbitMqQueue         = "RABBITMQ_QUEUE"
)

var (
	client             *mongo.Client
	ctx                context.Context
	publishChannelAmqp *amqp.Channel
	consumeChannelAmqp *amqp.Channel
	amqpConnection     *amqp.Connection

	//go:embed reddit.xml
	xmlByte []byte
)

func init() {
	ctx = context.Background()
	client, _ = mongo.Connect(
		ctx,
		options.Client().ApplyURI(os.Getenv(mongoUri)),
	)

	amqpConnection, err := amqp.Dial(os.Getenv(rabbitMqUri))
	if err != nil {
		log.Fatalln(err)
	}
	publishChannelAmqp, _ = amqpConnection.Channel()
}

func GetFeedEntries(url string) ([]Entry, error) {

	fmt.Println("url: ", url)

	req, err := http.NewRequest("GET", "https://www.reddit.com/r/recipes/.rss", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Custom Agent")
	var defaultClient = http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(authority string, c *tls.Conn) http.RoundTripper{},
		},
	}
	resp, err := defaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	//bodyString := string(bodyBytes)
	//fmt.Println("\n\n\n\n bodyString ----:", bodyString)
	//fmt.Println("bodyString len ----:", len(bodyString))

	var feed Feed
	err = xml.Unmarshal(bodyBytes, &feed)
	if err != nil {
		return nil, err
	}

	return feed.Entries, nil
}

func ParserHandler(c *gin.Context) {

	var request Request
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	entries, err := GetFeedEntries(request.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "error while parsing the RSS fees",
		})
		return
	}

	collection := client.Database(
		os.Getenv(mongoDatabase)).Collection(collectionNameRecipes)

	for _, entry := range entries[2:] {
		collection.InsertOne(ctx, bson.M{
			"title":     entry.Title,
			"thumbnail": entry.Thumbnail.URL,
			"url":       entry.Link.Href,
		})
	}

	c.JSON(http.StatusOK, entries)
}

func Publish2RabbitMqHandler(c *gin.Context) {
	var request Request
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	data, _ := json.Marshal(request)
	err := publishChannelAmqp.Publish(
		"",
		os.Getenv(rabbitMqQueue),
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(data),
		},
	)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error while publishing to RabbitMQ",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
	})
}

func ConsumeRabbitMqMessage() {
	amqpConnection, err := amqp.Dial(os.Getenv(rabbitMqUri))
	if err != nil {
		log.Fatalln(err)
	}

	consumeChannelAmqp, _ = amqpConnection.Channel()
	msgs, err := consumeChannelAmqp.Consume(
		os.Getenv(rabbitMqQueue),
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
			var request Request
			json.Unmarshal(d.Body, &request)
			log.Println("RSS URL:", request.URL)
			entries, _ := GetFeedEntries(request.URL)
			collection := client.Database(
				os.Getenv(mongoDatabase)).Collection(collectionNameRecipes)

			for _, entry := range entries[2:] {
				collection.InsertOne(ctx, bson.M{
					"title":     entry.Title,
					"thumbnail": entry.Thumbnail.URL,
					"url":       entry.Link.Href,
				})
			}
		}
	}()
}

func main() {
	fmt.Printf("rss-parser")

	ConsumeRabbitMqMessage()
	defer amqpConnection.Close()
	defer consumeChannelAmqp.Close()

	router := gin.Default()
	router.POST("/parse", ParserHandler)
	router.POST("/publish", Publish2RabbitMqHandler)

	log.Printf(" [*] Waiting for message. To exit press CYRL+C")
	err := router.Run(":5500")
	if err != nil {
		log.Fatalln(err)
	}
}
