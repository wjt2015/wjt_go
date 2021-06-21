package main

import (
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

/**
逐步学习http标准库;
*/
const (
	BUF_SIZE = 1 << 20
)

/**
测试
curl "http://localhost:10000/archives/561?name='jim green'&age=20"
*/
func listen() {
	addr := ":10000"
	listener, err := net.Listen("tcp", addr)
	logrus.Infof("listener=%+v;err=%+v;", listener, err)
	buf := make([]byte, BUF_SIZE)
	for {
		conn, err := listener.Accept()
		remoteAddr := conn.RemoteAddr()
		logrus.Infof("conn=%+v;err=%+v;remoteAddr=%+v", conn, err, remoteAddr)

		n, err := conn.Read(buf)
		logrus.Infof("n=%d;err=%+v;recv_str=%+v;", n, err, string(buf))
	}
	logrus.Infof("listen finish!")
}

func ticker() {
	ticker := time.NewTicker(10 * time.Millisecond)
	exitC := make(chan int)
	start := time.Now()
	now := time.Now()
	prevNow := time.Now()
	go func() {
		var tickId uint64 = 0
		for {
			select {
			case <-ticker.C:
				now = time.Now()

				elapsed := (now.UnixNano() - start.UnixNano()) / (int64)(time.Millisecond)
				interval := (now.UnixNano() - prevNow.UnixNano()) / (int64)(time.Millisecond)
				logrus.Infof("tick_id=%v;elapsed=%vms;interval=%vms", tickId, elapsed, interval)
				atomic.AddUint64(&tickId, 1)

				prevNow = now
				break
			}

			if tickId > 200 {
				exitC <- 1
				return
			}
		}
	}()
	logrus.Infof("main wait!")
	v := <-exitC
	logrus.Infof("exit!v=%v", v)
}

/**
golang下载文件实例:
https://www.jb51.net/article/165076.htm
http://www.52codes.net/develop/shell/59006.html
 */
func httpClient()  {
	//url:="https://dl.google.com/go/go1.5.3.darwin-amd64.pkg"

	url:="https://download.cntv.cn/cbox/mac/ysyy_v1.2.2.2_1001_setup.dmg?spm=0.PsnVeUG5xmKc.E3nZUgkyxygK.5&file=ysyy_v1.2.2.2_1001_setup.dmg"

	resp, err := http.Get(url)
	logrus.Infof("resp=%+v;err=%+v;",resp,err)
	if err==nil{
		defer resp.Body.Close()
	}

	//fileName:="data/go1.5.3.pkg"
	fileName:="data/ysyy.dmg"
	//BUF_SIZE:=1<<10
	dest, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0777)
	if err==nil{
		defer dest.Close()
	}
	
	io.Copy(dest,resp.Body)
	
/*	buf:=make([]byte,BUF_SIZE)
	for{
		n, err := resp.Body.Read(buf)
		logrus.Infof("read;n=%v;err=%+v;",n,err)
		if err==io.EOF{
			logrus.Infof("read finish!")
			break
		}
	}*/
	
	logrus.Infof("httpClient finish!")
}


