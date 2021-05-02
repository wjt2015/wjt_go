package fastdfs
/**
参考:
https://gitee.com/linux2014/go-fastdfs_2
 */

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	slog "log"
	random "math/rand"
	"mime/multipart"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/smtp"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/astaxie/beego/httplib"
	mapset "github.com/deckarep/golang-set"
	_ "github.com/eventials/go-tus"
	jsoniter "github.com/json-iterator/go"
	"github.com/nfnt/resize"
	"github.com/radovskyb/watcher"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/sjqzhang/googleAuthenticator"
	"github.com/sjqzhang/goutil"
	log "github.com/sjqzhang/seelog"
	"github.com/sjqzhang/tusd"
	"github.com/sjqzhang/tusd/filestore"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util")

type FileInfo struct{
	Name string `json:"name"`
	ReName string `json:"rename"`
	Path string `json:"path"`
	Md5 string `json:"md5"`
	Size int64 `json:"size"`
	Peers []string `json:"peers"`
	Scene string `json:"scene"`
	TimeStamp int64 `json:timeStamp`
	OffSet int64 `json:"offset"`
	retry int
	op string
}

type FileLog struct{
	FileInfo *FileInfo
	FileName string
}

type WrapReqResp struct{
	w *http.ResponseWriter
	r *http.Request
	done chan bool
}

type JsonResult struct{
	Message string `json:"message"`
	Status string `json:"status"`
	Data interface{} `json:"data"`
}

type FileResult struct{
	Url string `json:"url"`
	Md5 string `json:"md5"`
	Path string `json:"path"`
	Domain string `json:"domain"`
	Scene string `json:"scene"`
	Size int64 `json:"size"`
	ModTime int64 `json:"mtime"`
	//compatibility
	Scenes string `json:"scenes"`
	Retmsg string `json:"retmsg"`
	Retcode int `json:"retcode"`
	Src string `json:"src"`
}

type Mail struct{
	User string `json:"user"`
	Password int64 `json:"password"`
	Host string `json:"host"`
}

type StatDateFileInfo struct{
	Date string `json:"date"`
	TotalSize int64 `json:"totalSize"`
	FileCount int64 `json:"fileCount"`
}

type GlobalConfig struct{
	Addr string `json:"addr"`
	Peers []string `json:"peers"`
	EnableHttps bool `json:"enable_https"`
	Group string `json:"group"`
	RenameFile bool `json:"rename_file"`
	ShowDir bool `json:"show_dir"`
	Extensions []string `json:"extensions"`
	RefreshInterval int `json:"refresh_interval"`
	EnableWebUpload bool `json:"enable_web_upload"`
	DownloadDomain string `json:"download_domain"`
	EnableCustomPath bool `json:"enable_custom_path"`
	Scenes []string `json:"scenes"`
	AlarmReceivers  []string `alarm_receivers`
	DefaultScene string `json:default_scene`
	Mail Mail `json:"mail"`
	AlarmUrl string `json:"alarm_url"`
	DownloadUseToken bool `json:"download_use_token"`




}




type Server struct{
	ldb *leveldb.DB
	logDB * leveldb.DB
	util *goutil.Common
	statMap *goutil.CommonMap
	sumMap *goutil.CommonMap
	rtMap *goutil.CommonMap





}




