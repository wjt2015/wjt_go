package main

import (
	"fmt"
	"strconv"
	"github.com/gocolly/colly/v2"
)

func main(){
	fmt.Println("hello golang!")
	spider()
	fmt.Println(strconv.Quote("abcd"))
}

func spider(){
	c := colly.NewCollector()

	// Find and visit all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		e.Request.Visit(e.Attr("href"))
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	c.Visit("http://go-colly.org/")
}


