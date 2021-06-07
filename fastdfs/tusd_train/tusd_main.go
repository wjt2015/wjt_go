package main

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"

	"github.com/tus/tusd/pkg/filestore"
	tusd "github.com/tus/tusd/pkg/handler"
)

func main()  {
	//serv()
	//strip()
	filestoreB()
}


func strip(){
	s1:="ABCCD"
	s2:="ABCCDEF"
	prefix:= strings.TrimPrefix(s2, s1)

	logrus.Infof("prefix=%s",prefix)


}

func init()  {
	logrus.SetReportCaller(true)
}


func serv() {
	// Create a new FileStore instance which is responsible for
	// storing the uploaded file on disk in the specified directory.
	// This path _must_ exist before tusd will store uploads in it.
	// If you want to save them on a different medium, for example
	// a remote FTP server, you can implement your own storage backend
	// by implementing the tusd.DataStore interface.
	store := filestore.FileStore{
		Path: "./data/tusd_uploads",
	}

	// A storage backend for tusd may consist of multiple different parts which
	// handle upload creation, locking, termination and so on. The composer is a
	// place where all those separated pieces are joined together. In this example
	// we only use the file store but you may plug in multiple.
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)

	// Create a new HTTP handler for the tusd server by providing a configuration.
	// The StoreComposer property must be set to allow the handler to function.
	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:              "/files/",
		StoreComposer:         composer,
		NotifyCompleteUploads: true,
	})
	if err != nil {
		panic(fmt.Errorf("Unable to create handler: %s", err))
	}

	logrus.Infof("handlerType=%+v",reflect.ValueOf(handler))

	// Start another goroutine for receiving events from the handler whenever
	// an upload is completed. The event will contains details about the upload
	// itself and the relevant HTTP request.
	go func() {
		for {
			event := <-handler.CompleteUploads
			fmt.Printf("Upload %s finished\n", event.Upload.ID)
		}
	}()

	// Right now, nothing has happened since we need to start the HTTP server on
	// our own. In the end, tusd will start listening on and accept request at
	// http://localhost:8080/files
	h := http.StripPrefix("/files/", handler)

	logrus.Infof("h.type=%+v",reflect.ValueOf(h).Type())

	http.Handle("/files/",h )

	port:=8090
	logrus.Infof("http server listen on port=%d",port)
	err = http.ListenAndServe(fmt.Sprintf(":%d",port), nil)
	if err != nil {
		panic(fmt.Errorf("Unable to listen: %s", err))
	}

	logrus.Infof("tusd_main finish!")
}


func filestoreB(){
	//参考filestore.go,filestore_test.go,测试学习;
	dir:="./data/tusd_upload"
	store:=filestore.FileStore{
		Path: dir,
	}
	logrus.Infof("store=%+v",store)
	ctx:=context.Background()
	upload, err := store.NewUpload(ctx, tusd.FileInfo{
		Size: 42,
		MetaData: map[string]string{
			"hello": "world",
		},
	})

	logrus.Infof("upload=%+v;err=%+v;",upload,err)

	info, err := upload.GetInfo(ctx)
	logrus.Infof("info=%+v;err=%+v;",info,err)

	writeChunk, err := upload.WriteChunk(ctx, 0, strings.NewReader("hello world"))
	logrus.Infof("writeChunk=%+v;err=%+v;",writeChunk,err)

	info,err=upload.GetInfo(ctx)
	logrus.Infof("new_info=%+v;err=%+v;",info,err)

	reader, err := upload.GetReader(ctx)
	content, err := ioutil.ReadAll(reader)
	logrus.Infof("context=%+v;err=%+v;",string(content),err)
	if rc,ok:=reader.(io.Closer);ok{
		rc.Close()
	}
	info,err=upload.GetInfo(ctx)
	logrus.Infof("new2_info=%+v;err=%+v;",info,err)
	newUpload, err := store.GetUpload(ctx, info.ID)
	logrus.Infof("newUpload=%+v;err=%+v;",newUpload,err)
}
