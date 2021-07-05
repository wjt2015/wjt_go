package main

import (
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetReportCaller(true)
}

func main() {
	//httpServe()
	//chanFunc()
	//ctxFunc()
	//ctxFuncB()
	//listen()
	//ticker()
	//httpClient()
	transport()
}
