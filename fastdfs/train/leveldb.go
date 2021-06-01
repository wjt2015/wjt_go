package main

import (
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

func init(){
	logrus.SetReportCaller(true)
	logrus.Infof("init finish!")
}

func main(){

	db,err:=leveldb.OpenFile("wjt_leveldb",nil)
	if err!=nil{
		logrus.Panicf("openfile error!err=%+v",err)
	}
	defer db.Close()

	k:="golang"
	k2:="java"

	if err=db.Put([]byte(k),[]byte("为服务和云计算"),nil);err!=nil{
		logrus.Panicf("put error!err=%+v",err)
	}


	v,err:=db.Get([]byte(k),nil)
	if err!=nil{
		logrus.Errorf("Get k=%+v error!err=%+v",k,err)
	}
	logrus.Infof("k,v=>(%s,%s)",k,string(v))

	v,err=db.Get([]byte(k2),nil)
	if err!=nil{
		logrus.Errorf("Get k=%+v error!err=%+v",k2,err)
	}
	logrus.Infof("k,v=>(%s,%s)",k2,string(v))

	snapshot, err := db.GetSnapshot()
	logrus.Infof("snapshot=%+v;err=%+v;",snapshot,err)

}






