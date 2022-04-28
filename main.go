package main

import (
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/xml"
	"fmt"
	"github.com/gin-gonic/gin"
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
)

var (
	client *mongo.Client
	ctx    context.Context

	//go:embed reddit.xml
	xmlByte []byte
)

func init() {
	ctx = context.Background()
	client, _ = mongo.Connect(
		ctx,
		options.Client().ApplyURI(os.Getenv(mongoUri)),
	)
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

func main() {
	fmt.Printf("rss-parser")

	router := gin.Default()
	router.POST("/parse", ParserHandler)
	err := router.Run(":5500")
	if err != nil {
		log.Fatalln(err)
	}

}
