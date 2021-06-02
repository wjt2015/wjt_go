package main

import (
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"strconv"
)

func init(){
	logrus.SetReportCaller(true)
	logrus.Infof("init finish!")
}

func main(){

	db,err:=leveldb.OpenFile("data/wjt_leveldb",nil)
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

	batch(db);

}

func batch(db *leveldb.DB){

	batch:=&leveldb.Batch{
	}
	var i,j,n int
	i=0
	j=0
	n=10
	for i<n {
		k:=[]byte(strconv.Itoa(i))
		v:=[]byte(strconv.Itoa(j))
		batch.Put(k,v)
		i++
		j++
	}
	batch.Put([]byte(strconv.Itoa(3)),[]byte(strconv.Itoa(3)))
	db.Write(batch,nil)

	db.Put([]byte(strconv.Itoa(3)),[]byte(strconv.Itoa(3)),nil)

	logrus.Infof("start show:")
	 r := &util.Range{
		Start: []byte(strconv.Itoa(2)),
		Limit: []byte(strconv.Itoa(8)),
	}
	it := db.NewIterator(r, nil)
	defer it.Release()
	for it.Next(){
		k:=it.Key()
		v:=it.Value()
		logrus.Infof("k=%+v;v=%+v;",k,v)
	}

	logrus.Infof("finish show!")

}


func bloomFilter(){

	fileName:="data/wjt_leveldb_bloom_filter"
	options:=opt.Options{
		Filter: filter.NewBloomFilter(10),
	}
	db, err := leveldb.OpenFile(fileName, &options)
	if err!=nil{
		logrus.Panicf("open file error!err=%+v",err)
	}
	defer db.Close()


}





