package main

import (
	"github.com/sirupsen/logrus"
	"net"
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
			}
		}
	}()
	logrus.Infof("main wait!")
	v := <-exitC
	logrus.Infof("exit!v=%v", v)
}
