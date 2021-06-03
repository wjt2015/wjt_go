package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
)

func init()  {
	logrus.SetReportCaller(true)
}

type MyHttpHandler struct{}

func (httpHandler* MyHttpHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request){

	logrus.Infof("req=%+v;resp=%+v;",req,resp)

	resp.Write([]byte("golang http 测试!"))
}

func main(){
	httpServe()
}

func httpServe()  {

	port:=60000
	httpServer := http.Server{
		Addr: fmt.Sprintf(":%d",port),
		Handler: &MyHttpHandler{},
	}
	logrus.Infof("http server listen on port=%d",port)
	httpServer.ListenAndServe()

}






