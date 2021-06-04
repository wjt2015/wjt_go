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
	logrus.Infof("cookies=%+v;header=%+v;",req.Cookies(),req.Header)
	logrus.Infof("URL=%+v;requestURI=%+v;",req.URL,req.RequestURI)

	respHeader := resp.Header()
	respHeader.Add("golang","区块链,中间件")
	respHeader.Add("java","企业应用后端")

	resp.Write([]byte("golang http 测试!"))
}

func main(){
	httpServe()
}

func httpServe()  {

	port:=60000
	httpServer := http.Server{
		Addr: fmt.Sprintf(":%d",port),
		//Handler: &MyHttpHandler{},
	}
	defer httpServer.Close()
	logrus.Infof("http server listen on port=%d",port)
	uri:="/query.json"
	http.HandleFunc(uri,func(resp http.ResponseWriter,req *http.Request){
		logrus.Infof("uri=%s",uri)
	})
	uri="create.json"
	http.HandleFunc(uri,func(resp http.ResponseWriter,req *http.Request){
		logrus.Infof("uri=%s",uri)
	})

	httpServer.ListenAndServe()
	logrus.Infof("http server close on port=%d",port)
}






