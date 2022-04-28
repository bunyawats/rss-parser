package main

import (
	"crypto/tls"
	_ "embed"
	"encoding/xml"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
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

//go:embed reddit.xml
var xmlByte []byte

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
