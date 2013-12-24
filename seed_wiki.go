package main

import (
	"fmt"
	"io/ioutil"
	"bytes"
	"math/rand"
	"strconv"
	"time"
	"encoding/json"
	"net/http"
)

type Page struct {
	Key	string
	Title string
	Body	string
	Date	time.Time
}

func main() {
	//First, reset the database
	if resp, err := http.Get("http://localhost:8080/reset"); err != nil {
		fmt.Println("Error	Reset: " + err.Error())
	} else {
		printResponse(resp)
	}

	//Then create 20 new "Red" pages
	for i:=0; i<25; i++ {
		page := Page{Title: "Red", Body: "#"+strconv.Itoa(i)}
		jsonBytes, _ := json.Marshal(page)
		postPage(jsonBytes)
	}

	time.Sleep(500 * time.Millisecond)

	//While there are pages, pick a random page and change it's colour or delete it
	pages := getPages()
	for len(pages) > 0 {
		page := pages[rand.Intn(len(pages))]
		processPage(page)
		pages = getPages()
	}
}

func processPage(page Page) {
	switch page.Title {
	case "Red":
		page.Title = "Yellow"
		jsonBytes, _ := json.Marshal(page)
		postPage(jsonBytes)
	case "Yellow":
		page.Title = "Blue"
		jsonBytes, _ := json.Marshal(page)
		postPage(jsonBytes)
	case "Blue":
		deletePage(page)
	}
}

func deletePage(page Page) {
	if resp, err := http.Get("http://localhost:8080/delete/"+page.Key); err != nil {
		fmt.Println("Error	Delete	HTTP: " + err.Error())
	} else {
		printResponse(resp)
	}
}

func getPages() []Page {
	if resp, err := http.Get("http://localhost:8080/get/"); err != nil {
		fmt.Println("Error	Get	HTTP: " + err.Error())
	} else {
		defer resp.Body.Close()
		if body, err := ioutil.ReadAll(resp.Body); err != nil {
			fmt.Println("Error	Get	IO/Read: " + err.Error())
		} else {
			var pages []Page
			if err := json.Unmarshal(body, &pages); err != nil {
				fmt.Println("Error	Get	JSON: " + err.Error())
			} else {
				return pages
			}
		}
	}
	return nil
}

func postPage(payload []byte) {
	buffer := bytes.NewBuffer(payload)
	if resp, err := http.Post("http://localhost:8080/post/", "application/json", buffer); err != nil {
		fmt.Println("Error	Post	HTTP: " + err.Error())
	} else {
		printResponse(resp)
	}
}

func printResponse(resp *http.Response) {
	defer resp.Body.Close()
	if body, err := ioutil.ReadAll(resp.Body); err != nil {
		fmt.Println("Error	Post	IO/Read: " + err.Error())
	} else {
		fmt.Println(string(body))
	}
}
