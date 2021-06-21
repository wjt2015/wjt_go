package main

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

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
	//这里应用了context.Context;
	httpServer.ListenAndServe()
	logrus.Infof("http server close on port=%d",port)
}

func chanFunc(){
	n:=10
	msgs:=make(chan int,n)
	done:=make(chan bool)
	defer close(msgs)

	go func(){
		ticker:=time.NewTicker(10*time.Millisecond)
		for v:=range ticker.C{
			logrus.Infof("c_time=%+v;",v)
			select {
			case tv,ok:=<-done:
				logrus.Infof("child interrupt!tv=%+v;ok=%+v;",tv,ok)
				return
			default:
				logrus.Infof("default!msgs=%+v",msgs)
				logrus.Infof("send msg!v=%+v",<-msgs)
			}
		}
	}()

	//producer
	for i:=0;i<n;i++{
		msgs<-i
	}

	time.Sleep(10*time.Second)
	close(done)
	time.Sleep(1*time.Second)
	logrus.Infof("main return!")
}

func targetFunc(ctx context.Context){
	time.Sleep(1*time.Second)
	logrus.Infof("---;targetFunc")
}

func ctxFunc(){
	parentCtx:=context.Background()
	ctx,cancelFunc:=context.WithTimeout(parentCtx,3*time.Second)

	deadline, ok := ctx.Deadline()
	logrus.Infof("deadline=%+v;ok=%+v;",deadline,ok)
	cancelFunc()

	go targetFunc(ctx)

	b:=true
	for b{
		select {
		case <- ctx.Done():
			errV,ok:=ctx.Err().(error)
			logrus.Infof("errV=%+v;ok=%+v;",errV,ok)

			if ok{
				if errV==context.DeadlineExceeded{
					logrus.Infof("context.DeadlineExceeded")
					//return
				}else if errV==context.Canceled{
					logrus.Infof("context.Canceled")
					//return
				}else{
					logrus.Infof("done for other reason")
				}
			}

			b=false
		default:
			time.Sleep(time.Second)
			logrus.Infof("sleep 1s!")
		}//select
	}//for
	time.Sleep(3*time.Second)

	deadline, ok = ctx.Deadline()
	logrus.Infof("deadline=%+v;ok=%+v;",deadline,ok)

	logrus.Infof("ctxFunc() finish!")
}


func ctxFuncB(){
	parentCtx:=context.Background()
	deadline,ok:=parentCtx.Deadline()
	logrus.Infof("parentCtx=%+v;deadline=%+v;ok=%+v;Done=%+v;",parentCtx,deadline,ok,parentCtx.Done())

	withTimeoutCtx, cancelFunc := context.WithTimeout(parentCtx, 8*time.Second)
	logrus.Infof("withTimeoutCtx=%+v;cancelFunc=%+v;",withTimeoutCtx,cancelFunc)

	bTimeoutCtx, bCancelFunc := context.WithTimeout(withTimeoutCtx, 5*time.Second)
	logrus.Infof("bTimeoutCtx=%+v;bCancelFunc=%+v;",bTimeoutCtx, bCancelFunc)

	go func(){
		//任务A;
		stop:=false
		for !stop{
			logrus.Infof("taskA is doing!err=%+v;",withTimeoutCtx.Err())
			select {
			case <-withTimeoutCtx.Done():
				logrus.Infof("taskA stop!err=%+v;",withTimeoutCtx.Err())
				stop=true
				break
			default:
				//logrus.Infof("taskA sleeping!")
				time.Sleep(1*time.Second)
			}
		}
	}()

	go func(){
		//taskB
		stop:=false
		for !stop{
			logrus.Infof("taskB is doing!err=%+v;",bTimeoutCtx.Err())
			select {
			case <-bTimeoutCtx.Done():
				logrus.Infof("taskB stop!err=%+v;",bTimeoutCtx.Err())
				stop=true
				break
			default:
				time.Sleep(1*time.Second)
				break
			}
		}
	}()
	time.Sleep(2*time.Second)
	cancelFunc()

	time.Sleep(10*time.Second)

	logrus.Infof("ctxFuncB finish!")
}



