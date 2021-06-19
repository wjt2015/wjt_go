package main

import (
	"github.com/sirupsen/logrus"
	"net"
)

/**
逐步学习http标准库;
 */
const (
	BUF_SIZE=1<<20;
)

/**
测试
curl "http://localhost:10000/archives/561?name='jim green'&age=20"
 */
func listen(){
	addr:=":10000"
	listener, err := net.Listen("tcp", addr)
	logrus.Infof("listener=%+v;err=%+v;",listener,err)
	buf:=make([]byte,BUF_SIZE)
	for{
		conn, err := listener.Accept()
		remoteAddr := conn.RemoteAddr()
		logrus.Infof("conn=%+v;err=%+v;remoteAddr=%+v",conn,err,remoteAddr)

		n, err := conn.Read(buf)
		logrus.Infof("n=%d;err=%+v;recv_str=%+v;",n,err,string(buf))
	}
	logrus.Infof("listen finish!")
}


