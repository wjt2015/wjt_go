package fastdfs
/**
参考:
https://gitee.com/linux2014/go-fastdfs_2
 */

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	mapset "github.com/deckarep/golang-set"
	"github.com/nfnt/resize"
	"github.com/radovskyb/watcher"
	"github.com/sirupsen/logrus"
	"github.com/tdewolff/parse/v2/js"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"math/rand"
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
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/astaxie/beego/httplib"
	_ "github.com/eventials/go-tus"
	jsoniter "github.com/json-iterator/go"
	"github.com/sjqzhang/goutil"
	log "github.com/sjqzhang/seelog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

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
	Password string `json:"password"`
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
	DownloadTokenExpire int `json:"download_token_expire"`
	QueueSize int `json:"queue_size"`
	AutoRepair bool `json:"auto_repair"`
	Host string `json:"host"`
	FileSumArithmetic string `json:"file_sum_arithmetic"`
	PeerId string `json:"peer_id"`
	SupportGroupManage bool `json:"support_group_manage"`
	AdminIps []string `json:"admin_ips"`
	EnableMergeSmallFile bool `json:"enable_merge_small_file"`
	EnableMigrate bool `json:"enable_migrate"`
	EnableDistinctFile bool `json:"enable_distinct_file"`
	ReadOnly bool `json:read_only`
	EnableCrossOrigin bool `json:"enable_cross_origin"`
	EnableGoogleAuth bool `json:"enable_google_auth"`
	AuthUrl string `json:"auth_url"`
	EnableDownloadAuth bool `json:"enable_download_auth"`
	DefaultDownload bool `json:"default_download"`
	EnableTus bool `json:"enable_tus"`
	SyncTimeout int64 `json:"sync_timeout"`
	EnableFsnotify bool `json:"enable_fsnotify"`
	EnableDiskCache bool `json:"enable_disk_cache"`
	ConnectTimeout bool `json:"connect_timeout"`
	ReadTimeout int `json:"read_timeout"`
	WriteTimeout int `json:"write_timeout"`
	IdleTimeout int `json:"idle_timeout"`
	ReadHeaderTimeout int `json:"read_header_timeout"`
	SyncWorker int `json:"sync_worker"`
	UploadWorker int `json:"upload_worker"`
	UploadQueueSize int `json:"upload_queue_size"`
	RetryCount int `json:"retry_count"`
	SyncDelay int64 `json:"sync_delay"`
	WatchChanSize int `json:"watch_chan_size"`
}

type FileInfoResult struct{
	Name string `json:"name"`
	Md5 string `json:"md5"`
	Path string `json:"path"`
	Size int64 `json:"size"`
	ModTime int64 `json:"mtime"`
	IsDir bool `json:"is_dir"`
}


type Server struct{
	ldb *leveldb.DB
	logDB *leveldb.DB
	util *goutil.Common
	statMap *goutil.CommonMap
	sumMap *goutil.CommonMap
	rtMap *goutil.CommonMap
	queueToPeers chan FileInfo
	queueFromPeers chan FileInfo
	queueFileLog chan *FileLog
	queueUpload chan WrapReqResp
	lockMap *goutil.CommonMap
	sceneMap *goutil.CommonMap
	searchMap *goutil.CommonMap
	curDate string
	host string
}

var staticHandler http.Handler
var json=jsoniter.ConfigCompatibleWithStandardLibrary
var logacc log.LoggerInterface
var FOLDERS=[]string{DATA_DIR,STORE_DIR,CONF_DIR,STATIC_DIR}
var CONST_QUEUE_SIZE=10000
var server *Server=nil

var (
	VERSION string
	BUILD_TIME string
	GO_VERSION string
	GIT_VERSION string
	v=flag.Bool("v",false,"display version")
)

var (
	FileName string
	ptr unsafe.Pointer
	DOCKER_DIR=""
	STORE_DIR=STORE_DIR_NAME
	CONF_DIR=CONF_DIR_NAME
	LOG_DIR=LOG_DIR_NAME
	DATA_DIR=DATA_DIR_NAME
	STATIC_DIR=STATIC_DIR_NAME
	LARGE_DIR_NAME="haystack"
	LARGE_DIR=STORE_DIR+"/haystack"
	CONST_LEVELDB_FILE_NAME=DATA_DIR+"/fileserver.db"
	CONST_LOG_LEVELDB_FILE_NAME=DATA_DIR+"/log.db"
	CONST_STAT_FILE_NAME=DATA_DIR+"/stat.json"
	CONST_CONF_FILE_NAME=CONF_DIR+"/cfg.json"
	CONST_SERVER_CRT_FILE_NAME=CONF_DIR+"/server.crt"
	CONST_SERVER_KEY_FILE_NAME=CONF_DIR+"/server.key"
	CONST_SEARCH_FILE_NAME=DATA_DIR+"/search.txt"
	CONST_UPLOAD_COUNTER_KEY="__CONST_UPLOAD_COUNTER_KEY__"
	logConfigStr=`
<seelog type="asynctimer" asyncinterval="1000" minlevel="trace" maxlevel="error">  
	<outputs formatid="common">  
		<buffered formatid="common" size="1048576" flushperiod="1000">  
			<rollingfile type="size" filename="{DOCKER_DIR}log/fileserver.log" maxsize="104857600" maxrolls="10"/>  
		</buffered>
	</outputs>  	  
	 <formats>
		 <format id="common" format="%Date %Time [%LEV] [%File:%Line] [%Func] %Msg%n" />  
	 </formats>  
</seelog>
`
	logAccessConfigStr=`
<seelog type="asynctimer" asyncinterval="1000" minlevel="trace" maxlevel="error">  
	<outputs formatid="common">  
		<buffered formatid="common" size="1048576" flushperiod="1000">  
			<rollingfile type="size" filename="{DOCKER_DIR}log/access.log" maxsize="104857600" maxrolls="10"/>  
		</buffered>
	</outputs>  	  
	 <formats>
		 <format id="common" format="%Date %Time [%LEV] [%File:%Line] [%Func] %Msg%n" />  
	 </formats>  
</seelog>
`
)

const (
	STORE_DIR_NAME="files"
	LOG_DIR_NAME="log"
	DATA_DIR_NAME="data"
	CONF_DIR_NAME="conf"
	STATIC_DIR_NAME="static"
	CONST_STAT_FILE_COUNT_KEY="fileCount"
	CONST_BIG_UPLOAD_PATH_SUFFIX="/big/upload/"
	CONST_STAT_FILE_TOTAL_SIZE_KEY="totalSize"
	CONST_Md5_ERROR_FILE_NAME="errors.md5"
	CONST_Md5_QUEUE_FILE_NAME="queue.md5"
	CONST_FILE_Md5_FILE_NAME="files.md5"
	CONST_REMOVE_Md5_FILE_NAME="removes.md5"
	CONST_SMALL_FILE_SIZE=1024*1024
	CONST_MESSAGE_CLUSTER_IP="Can only be called by the cluster ip or 127.0.0.1 or admin_ips(cfg.json),current ip:%s"
	cfgJson=`
{
	"绑定端号": "端口",
	"addr": ":3600",
	"是否开启https": "默认不开启，如需启开启，请在conf目录中增加证书文件 server.crt 私钥 文件 server.key",
	"enable_https": false,
	"PeerID": "集群内唯一,请使用0-9的单字符，默认自动生成",
	"peer_id": "%s",
	"本主机地址": "本机http地址,默认自动生成(注意端口必须与addr中的端口一致），必段为内网，自动生成不为内网请自行修改，下同",
	"host": "%s",
	"集群": "集群列表,注意为了高可用，IP必须不能是同一个,同一不会自动备份，且不能为127.0.0.1,且必须为内网IP，默认自动生成",
	"peers": ["%s"],
	"组号": "用于区别不同的集群(上传或下载)与support_group_manage配合使用,带在下载路径中",
	"group": "group1",
	"是否支持按组（集群）管理,主要用途是Nginx支持多集群": "默认支持,不支持时路径为http://10.1.5.4:3600/action,支持时为http://10.1.5.4:3600/group(配置中的group参数)/action,action为动作名，如status,delete,sync等",
	"support_group_manage": true,
	"是否合并小文件": "默认不合并,合并可以解决inode不够用的情况（当前对于小于1M文件）进行合并",
	"enable_merge_small_file": false,
    "允许后缀名": "允许可以上传的文件后缀名，如jpg,jpeg,png等。留空允许所有。",
	"extensions": [],
	"重试同步失败文件的时间": "单位秒",
	"refresh_interval": 10,
	"是否自动重命名": "默认不自动重命名,使用原文件名",
	"rename_file": false,
	"是否支持web上传,方便调试": "默认支持web上传",
	"enable_web_upload": true,
	"是否支持非日期路径": "默认支持非日期路径,也即支持自定义路径,需要上传文件时指定path",
	"enable_custom_path": true,
	"下载域名": "用于外网下载文件的域名,不包含http://",
	"download_domain": "",
	"场景列表": "当设定后，用户指的场景必项在列表中，默认不做限制(注意：如果想开启场景认功能，格式如下：'场景名:googleauth_secret' 如 default:N7IET373HB2C5M6D ",
	"scenes": [],
	"默认场景": "默认default",
	"default_scene": "default",
	"是否显示目录": "默认显示,方便调试用,上线时请关闭",
	"show_dir": true,
	"邮件配置": "",
	"mail": {
		"user": "abc@163.com",
		"password": "abc",
		"host": "smtp.163.com:25"
	},
	"告警接收邮件列表": "接收人数组",
	"alarm_receivers": [],
	"告警接收URL": "方法post,参数:subject,message",
	"alarm_url": "",
	"下载是否需带token": "真假",
	"download_use_token": false,
	"下载token过期时间": "单位秒",
	"download_token_expire": 600,
	"是否自动修复": "在超过1亿文件时出现性能问题，取消此选项，请手动按天同步，请查看FAQ",
	"auto_repair": true,
	"文件去重算法md5可能存在冲突，默认md5": "sha1|md5",
	"file_sum_arithmetic": "md5",
	"管理ip列表": "用于管理集的ip白名单,",
	"admin_ips": ["127.0.0.1"],
	"是否启用迁移": "默认不启用",
	"enable_migrate": false,
	"文件是否去重": "默认去重",
	"enable_distinct_file": true,
	"是否开启跨站访问": "默认开启",
	"enable_cross_origin": true,
	"是否开启Google认证，实现安全的上传、下载": "默认不开启",
	"enable_google_auth": false,
	"认证url": "当url不为空时生效,注意:普通上传中使用http参数 auth_token 作为认证参数, 在断点续传中通过HTTP头Upload-Metadata中的auth_token作为认证参数,认证流程参考认证架构图",
	"auth_url": "",
	"下载是否认证": "默认不认证(注意此选项是在auth_url不为空的情况下生效)",
	"enable_download_auth": false,
	"默认是否下载": "默认下载",
	"default_download": true,
	"本机是否只读": "默认可读可写",
	"read_only": false,
	"是否开启断点续传": "默认开启",
	"enable_tus": true,
	"同步单一文件超时时间（单位秒）": "默认为0,程序自动计算，在特殊情况下，自已设定",
	"sync_timeout": 0
}
`
)




func NewServer() *Server{
	if server!=nil{
		return server
	}

	server:=&Server{
		util:&goutil.Common{},
		statMap:goutil.NewCommonMap(0),
		lockMap:goutil.NewCommonMap(0),
		rtMap:goutil.NewCommonMap(0),
		sceneMap:goutil.NewCommonMap(0),
		searchMap: goutil.NewCommonMap(0),
		queueToPeers: make(chan FileInfo,CONST_QUEUE_SIZE),
		queueFromPeers: make(chan FileInfo,CONST_QUEUE_SIZE),
		queueFileLog: make(chan *FileLog,CONST_QUEUE_SIZE),
		queueUpload: make(chan WrapReqResp,100),
		sumMap: goutil.NewCommonMap(365*3),
	}
	defaultTransport:=&http.Transport{
		DisableKeepAlives: true,
		Dial: httplib.TimeoutDialer(time.Second*15,time.Second*300),
		MaxIdleConns:100,
		MaxIdleConnsPerHost:100,
	}
	settings:=httplib.BeegoHTTPSettings{
		UserAgent: "Go-FastDFS",
		ConnectTimeout: 15*time.Second,
		ReadWriteTimeout: 15*time.Second,
		Gzip:true,
		DumpBody: true,
		Transport: defaultTransport,
	}

	httplib.SetDefaultSetting(settings)

	server.statMap.Put(CONST_STAT_FILE_COUNT_KEY,int64(0))
	server.statMap.Put(CONST_STAT_FILE_TOTAL_SIZE_KEY,int64(0))
	server.statMap.Put(server.util.GetToDay()+"_"+CONST_STAT_FILE_COUNT_KEY,int64(0))
	server.statMap.Put(server.util.GetToDay()+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY,int64(0))
	server.curData=server.util.GetToDay()

	opts:=&opt.Options{
		CompactionTableSize: 1024*1024*20,
		WriteBuffer: 1024*1024*20,
	}
	var err error
	server.logDB,err=leveldb.OpenFile(CONST_LOG_LEVELDB_FILE_NAME,opts)
	if err!=nil{
		logrus.Errorf("open leveldb file %s fail,maybe has been opening!err:=%+v\n",CONST_LOG_LEVELDB_FILE_NAME,err)
		panic(err)
	}

	return server
}

//need test;
func Config() *GlobalConfig{
	return (*GlobalConfig)(atomic.LoadPointer(&ptr))
}
/**
用默认的配置或指定的配置文件;
 */
func ParseConfig(filePath string) {
	var data []byte
	if filePath==""{
		data=[]byte(strings.TrimSpace(cfgJson))
	}else {
		file,err:=os.Open(filePath)

		if err!=nil{
			panic(fmt.Sprintln("open file path:",filePath,"error:",err))
		}
		defer file.Close()
		FileName=filePath
		data,err=ioutil.ReadAll(file)
		if err!=nil{
			panic(fmt.Sprintln("file path:",filePath," read all error: ",err))
		}

	}
	var c GlobalConfig
	if err:=json.Unmarshal(data,&c);err!=nil{
		panic(fmt.Sprintln("file path:",filePath," json unmarshal error:",err))
	}
	logrus.Infof("c=%+v\n",c)
	atomic.StorePointer(&ptr,unsafe.Pointer(&c))
	logrus.Infof("config parse success!")
}

/**
need test
按日期备份元数据;
 */
func (s *Server) BackUpMetaDataByDate(date string){
	defer func(){
		if re:=recover();re!=nil{
			buffer:=debug.Stack()
			logrus.Errorf("BackUpMetaDataByDate!re:%+v;buffer:%s\n",re,string(buffer))
		}
	}()

	var (
		err error
		keyPrefix string
		msg string
		name string
		fileInfo FileInfo
		logFileName string
		fileLog *os.File
		fileMeta *os.File
		metaFileName string
		fi os.FileInfo
	)
	logFileName=DATA_DIR+"/"+date+"/"+CONST_FILE_Md5_FILE_NAME
	s.lockMap.LockKey(logFileName)
	defer s.lockMap.UnLockKey(logFileName)
	metaFileName=DATA_DIR+"/"+date+"/"+"meta.data"
	os.MkdirAll(DATA_DIR+"/"+date,0775)

    if s.util.IsExist(logFileName){
    	os.Remove(logFileName)
	}

	if s.util.IsExist(metaFileName){
		os.Remove(metaFileName)
	}

	if fileLog,err=os.OpenFile(logFileName,os.O_RDWR|os.O_CREATE|os.O_APPEND,0664);err!=nil{
		logrus.Errorf("openFile(%s) error(%+v)\n",logFileName,err)
		return
	}
	defer fileLog.Close()

	if fileMeta,err=os.OpenFile(metaFileName,os.O_RDWR|os.O_CREATE|os.O_APPEND,0664);err!=nil{
		logrus.Errorf("openFile(%s) error(%+v)\n",logFileName,err)
		return
	}
	defer fileMeta.Close()

	keyPrefix=fmt.Sprintf("%s_%s",date,CONST_FILE_Md5_FILE_NAME)
	it:=server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)),nil)
	defer it.Release()

	for it.Next(){
		if err=json.Unmarshal(it.Value(),&fileInfo);err!=nil{
			logrus.Errorf("unmarshal error!it=%+v\n",it)
			continue
		}
		if fileInfo.ReName!=""{
			name=fileInfo.ReName
		}else {
			name=fileInfo.Name
		}
		msg=fmt.Sprintf("%s\t%s\n",fileInfo.Md5,string(it.Value()))
		if _,err=fileMeta.WriteString(msg);err!=nil{
			logrus.Errorf("fileMeta(%+v) write error!msg=%s;err=%+v\n",fileMeta,msg,err)
		}

		msg=fmt.Sprintf("%s\t%s\n",s.util.MD5(fileInfo.Path+"/"+name),string(it.Value()))
		if _,err=fileMeta.WriteString(msg);err!=nil{
			logrus.Errorf("fileMeta(%+v) write error!msg=%s;err=%+v\n",fileMeta,msg,err)
		}

		msg=fmt.Sprintf("%s|%d|%d|%s\n",fileInfo.Md5,fileInfo.Size,fileInfo.TimeStamp,fileInfo.Path+"/"+name)
		if _,err=fileMeta.WriteString(msg);err!=nil{
			logrus.Errorf("fileMeta(%+v) write error!msg=%s;err=%+v\n",fileMeta,msg,err)
		}
	}

	if fi,err=fileLog.Stat();err!=nil{
		logrus.Errorf("fileLog stat error!err=%+v\n",err)
	}else if fi.Size()==0{
		fileLog.Close()
		os.Remove(logFileName)
	}

	if fi,err=fileMeta.Stat();err!=nil{
		logrus.Errorf("fileMeta stat error!err=%+v\n",err)
	}else if fi.Size()==0{
		fileMeta.Close()
		os.Remove(metaFileName)
	}
}

var globalServer *Server
var pathPrefix string

func handleFunc(filePath string,f os.FileInfo,err error) error{
	var (
		files []os.FileInfo
		fi os.FileInfo
		fileInfo FileInfo
		sum string
		pathMd5 string
	)
	if f.IsDir(){
		if files,err=ioutil.ReadDir(filePath);err!=nil{
			logrus.Errorf("read_dir error!filePath=%s;err=%+v\n",filePath,err)
			return err
		}
		for _,fi=range files{
			if fi.Size()==0||fi.IsDir(){
				continue
			}
			filePath=strings.Replace(filePath,"\\","/",-1)
			if DOCKER_DIR!=""{
				filePath=strings.Replace(filePath,DOCKER_DIR,"",1)
			}
			if pathPrefix!=""{
				filePath=strings.Replace(filePath,pathPrefix,STORE_DIR_NAME,1)
			}
			if strings.HasPrefix(filePath,STORE_DIR_NAME+"/"+LARGE_DIR_NAME){
				logrus.Infof(fmt.Sprintf("ignore small file file %s!",filePath+"/"+fi.Name()))
				continue
			}
			pathMd5=globalServer.util.MD5(filePath+"/"+fi.Name())
			sum=pathMd5

			if err!=nil{
				logrus.Errorf("err=%+v\n",err)
				continue
			}
			fileInfo=FileInfo{
				Size:fi.Size(),
				Name:fi.Name(),
				Path:filePath,
				Md5:sum,
				TimeStamp: fi.ModTime().Unix(),
				Peers: []string{globalServer.host},
				OffSet: -2,
			}
			logrus.Infof("fileInfo=%+v\n",fileInfo)
			//todo
			//s.AppendToQueue(&fileInfo)
			//s.postFileToPeer(&fileInfo)
			//s.SaveFileInfoToLevelDB(fileInfo.Md5,&fileInfo,s.ldb)
			//s.SaveFileMd5Log(&fileInfo,CONST_FILE_Md5_FILE_NAME)
		}
	}
	return nil
}

func (s *Server) RepairFileInfoFromFile(){
	var (
		//pathPrefix string
		err error
		fi os.FileInfo
	)
	globalServer=s

	defer func(){
		if re:=recover();re!=nil{
			buffer:=debug.Stack()
			logrus.Errorf("RepairFileInfoFromFile error!re=%+v;buffer=%s\n",re,string(buffer))
		}
	}()

	if s.lockMap.IsLock("RepairFileInfoFromFile"){
		logrus.Warnf("Lock RepairFileInfoFromFile")
		return
	}

	s.lockMap.LockKey("RepairFileInfoFromFile")
	defer s.lockMap.UnLockKey("RepairFileInfoFromFile")
	//handlefunc
	pathname:=STORE_DIR
	if pathPrefix,err=os.Readlink(pathname);err==nil{
		pathname=pathPrefix

		if strings.HasSuffix(pathPrefix,"/"){
			pathPrefix=pathPrefix[0:len(pathPrefix)-1]
		}
	}
	if fi,err=os.Stat(pathname);err!=nil{
		logrus.Errorf("stat error!pathname=%s;err=%+v;\n",pathname,err)
	}
	if fi.IsDir(){
		filepath.Walk(pathname,handleFunc)
	}
	logrus.Infof("RepairFileInfoFromFile finish!")
}

func (s *Server) WatchFilesChange(){
	var (
		w *watcher.Watcher
		curDir string
		err error
		qchan chan *FileInfo
		isLink bool
	)
	qchan=make(chan *FileInfo,Config().WatchChanSize)
	w=watcher.New()
	w.FilterOps(watcher.Create)

	if curDir,err=filepath.Abs(filepath.Dir(STORE_DIR_NAME));err!=nil{
		logrus.Errorf("file error!err=%+v\n",err)
	}
	go func(){
		//事件监控协程;
		for{
			select{
			case event :=<-w.Event:
				logrus.Infof("event=%+v\n",event)
				if event.IsDir(){
					continue
				}
				fpath:=strings.Replace(event.Path,curDir+string(os.PathSeparator),"",1)

				if isLink{
					fpath=strings.Replace(event.Path,curDir,STORE_DIR_NAME,1)
				}
				fpath=strings.Replace(fpath,string(os.PathSeparator),"/",-1)
				sum:=s.util.MD5(fpath)
				fileInfo:=FileInfo{
					Size:event.Size(),
					Name:event.Name(),
					Path:strings.TrimSuffix(fpath,"/"+event.Name()),
					Md5:sum,
					TimeStamp: event.ModTime().Unix(),
					Peers: []string{s.host},
					OffSet: -2,
					op:event.Op.String(),
				}
				logrus.Infof(fmt.Sprintf("WatchFilesChange op:%s path:%s",event.Op.String(),fpath))
				//一旦有事件发生,则将fileInfo加入qchan;
				qchan <- &fileInfo
				break
			case err =<- w.Error:
				logrus.Errorf("err=%+v\n",err)
				break
			case v:=<-w.Closed:
				logrus.Infof("close!v=%+v\n",v)
				return
			}//select;
		}//for
	}()//go func();

	go func(){
		//处理qchan内的事件;
		for{
			c:=<-qchan
			if time.Now().Unix()-c.TimeStamp<Config().SyncDelay{
				qchan <- c
				time.Sleep(time.Second)
				continue
			}else {
				if c.op==watcher.Create.String(){
					logrus.Infof(fmt.Sprintf("Syncfile Add to queue path:%s\n",c.Path+"/"+c.Name))
					//todo
					//s.AppendToQueue(c)
					//s.SaveFileInfoToLevelDB(c.Md5,c,s.ldb)
				}
			}
		}//for
	}()//go func()

	if dir,err:=os.Readlink(STORE_DIR_NAME);err==nil{
		if strings.HasSuffix(dir,string(os.PathSeparator)){
			dir=strings.TrimSuffix(dir,string(os.PathSeparator))
		}
		curDir=dir
		isLink=true
		if err:=w.AddRecursive(dir);err!=nil{
			logrus.Errorf("AddRecursive err=%+v\n",v)
		}
		w.Ignore(dir+"/_tmp/")
		w.Ignore(dir+"/"+LARGE_DIR_NAME+"/")
	}
	if err:=w.AddRecursive("./"+STORE_DIR_NAME);err!=nil{
		logrus.Errorf("AddRecursive err=%+v\n",v)
	}
	w.Ignore("./"+STORE_DIR_NAME+"/_tmp/")
	w.Ignore("./"+STORE_DIR_NAME+"/"+LARGE_DIR_NAME+"/")
	if err:=w.Start(time.Millisecond*100);err!=nil{
		logrus.Errorf("watcher start error!err=%+v\n",err)
	}
}

func (s *Server) RepairStatByDate(date string) StatDateFileInfo{
	defer func(){
		if re:=recover();re!=nil{
			buffer:=debug.Stack()
			logrus.Errorf("RepairStatByDate;re=%+v;buffer=%+v\n",re,string(buffer))
		}
	}()
	var (
		err error
		keyPrefix string
		fileInfo FileInfo
		fileCount int64
		fileSize int64
		stat StatDateFileInfo
	)
	keyPrefix=fmt.Sprintf("%s_%s_",date,CONST_FILE_Md5_FILE_NAME)
	it:=s.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)),nil)
	defer it.Release()
	for it.Next(){
		if err=json.Unmarshal(it.Value(),&fileInfo);err!=nil{
			continue
		}
		fileCount++
		fileSize+=fileInfo.Size
	}
	s.statMap.Put(date+"_"+CONST_STAT_FILE_COUNT_KEY,fileCount)
	s.statMap.Put(date+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY,fileSize)
	//todo
	//s.SaveStat()
	stat.Date=date
	stat.FileCount=fileCount
	stat.TotalSize=fileSize
	return stat
}

func (s *Server) GetFilePathByInfo(fileInfo *FileInfo,withDocker bool) string{
	fn:=fileInfo.Name
	if fileInfo.ReName!=""{
		fn=fileInfo.ReName
	}
	if withDocker{
		return DOCKER_DIR+fileInfo.Path+"/"+fn
	}
	return fileInfo.Path+"/"+fn
}

func (s *Server) CheckFileExistByInfo(md5s string,fileInfo *FileInfo) bool{
	var (
		err error
		fullPath string
		fi os.FileInfo
		info *FileInfo
	)
	if fileInfo==nil{
		return false
	}

	if fileInfo.OffSet>=0{
		//small file;
		/**
		if info,err=s.GetFileInfoFromLevelDB(fileInfo.Md5);err==nil&&info.Md5==fileInfo.Md5{
		   return true
		}else{
		   return false
		}
		 */
	}

	fullPath=s.GetFilePathByInfo(fileInfo,true)
	if fi,err=os.Stat(fullPath);err!=nil{
		return false
	}
	if fi.Size()==fileInfo.Size{
		return true
	}else {
		return false
	}
}

func (s *Server) ParseSmallFile(fileName string) (string,int64,int,error){
	var (
		err error
		offset int64
		length int
	)
	err=errors.New("invalid small file")
	if len(fileName)<3{
		return fileName,-1,-1,err
	}
	if strings.Contains(fileName,"/"){
		fileName=fileName[strings.LastIndex(fileName,"/"):]
	}
	pos:=strings.Split(fileName,",")
	if len(pos)<3{
		return fileName,-1,-1,err
	}
	if offset,err=strconv.ParseInt(pos[1],10,64);err!=nil{
		return fileName,-1,-1,err
	}
	if length,err=strconv.Atoi(pos[2]);err!=nil{
		return fileName,offset,-1,err
	}
	if length>CONST_SMALL_FILE_SIZE||offset<0{
		err=errors.New("invalid filesize pf offset")
		return fileName,-1,-1,err
	}
	return pos[0],-1,-1,err
}


func (s *Server) DownloadFromPeer(peer string,fileInfo *FileInfo){
	var(
		err error
		fileName string
		fpath string
		fpathTmp string
		fi os.FileInfo
		sum string
		data []byte
		downloadUrl string
	)
	if Config().ReadOnly{
		logrus.Warnf("Readonly; fileInfo=%+v\n",fileInfo)
		return
	}
	if Config().RetryCount>0&&fileInfo.retry>=Config().RetryCount{
		logrus.Errorf("DownloadFromPeer error!fileInfo=%+v\n",fileInfo)
		return
	}else {
		fileInfo.retry++
	}
	fileName=fileInfo.Name
	if fileInfo.ReName!=""{
		fileName=fileInfo.ReName
	}
	if fileInfo.OffSet!=-2&&Config().EnableDistinctFile&&s.CheckFileExistByInfo(fileInfo.Md5,fileInfo){
		//ignore migrate file
		logrus.Infof("DownloadFromPeer file exist;path=%+v\n",fileInfo.Path+"/"+fileInfo.Name)
		return
	}
	if (!Config().EnableDistinctFile||fileInfo.OffSet==-2)||s.util.FileExists(s.GetFilePathByInfo(fileInfo,true)){
		if fi,err=os.Stat(s.GetFilePathByInfo(fileInfo,true));err!=nil{
			logrus.Infof("ignore file sync path:%s\n",s.GetFilePathByInfo(fileInfo,false))
			fileInfo.TimeStamp=fi.ModTime().Unix()
			//to do
			//s.PostFileToPeer(fileInfo);
			return
		}
		os.Remove(s.GetFilePathByInfo(fileInfo,true))
	}

	if _,err=os.Stat(fileInfo.Path);err!=nil{
		os.MkdirAll(DOCKER_DIR+fileInfo.Path,0775)
	}

	p:=strings.Replace(fileInfo.Path,STORE_DIR_NAME+"/","",1)

	if Config().SupportGroupManage{
		downloadUrl=peer+"/"+Config().Group+"/"+p+"/"+fileName
	}else {
		downloadUrl=peer+"/"+p+"/"+fileName
	}
	logrus.Infof("DownloadFromPeer url=%s\n",downloadUrl)
	fpath=DOCKER_DIR+fileInfo.Path+"/"+fileName
	fpathTmp=DOCKER_DIR+fileInfo.Path+"/tmp_"+fileName
	timeout:=fileInfo.Size>>20+30
	if Config().SyncTimeout>0{
		timeout=Config().SyncTimeout
	}
	s.lockMap.LockKey(fpath)
	defer s.lockMap.UnLockKey(fpath)
	downloadKey:=fmt.Sprintf("downloading_%d_%s",time.Now().Unix(),fpath)
	s.ldb.Put([]byte(downloadKey),[]byte(""),nil)
	defer func(){
		s.ldb.Delete([]byte(downloadKey),nil)
	}()

	if fileInfo.OffSet==-2{
		//migrate
		if fi,err=os.Stat(fpath);err==nil&&fi.Size()==fileInfo.Size{
			//todo
			//s.SaveFileInfoTo:LevelDB(fileInfo.Md5,fileInfo,fpath)
			return
		}
		req:=httplib.Get(downloadUrl)
		req.SetTimeout(time.Second*30,time.Second*time.Duration(timeout))
		if err=req.ToFile(fpathTmp);err!=nil{
			//todo
			//s.AppendToDowndQueue(fileInfo);//retry
			os.Remove(fpathTmp)
			logrus.Errorf("req.ToFile error!fpathTmp=%s;err=%+v;",fpathTmp,err)
			return
		}

		if fi,err=os.Stat(fpathTmp);err!=nil{
			os.Remove(fpathTmp)
			return
		}else if fi.Size()!=fileInfo.Size{
			logrus.Errorf("file size check error!fi.Name=%s;fileInfo.Name=%s;",fi.Name(),fileInfo.Name)
			os.Remove(fpathTmp)
		}
		if os.Rename(fpathTmp,fpath)==nil{
			//todo
			//s.SaveFileInfoToLevelDB(fileInfo.Md5,fileInfo,s.ldb)
		}
		return
	}
	req:=httplib.Get(downloadUrl)
	req.SetTimeout(time.Second*30,time.Second*time.Duration(timeout))

	if fileInfo.OffSet>=0{
		//small file download
		if data,err=req.Bytes();err!=nil{
			//todo
			//s.AppendToDownloadQueue(fileInfo)
			logrus.Errorf("download error!err=%+v\n",err)
			return
		}
		data2:=make([]byte,len(data)+1)
		data2[0]='1'
		for i,v:=range data{
			data2[i+1]=v
		}
		data=data2

		if int64(len(data))!=fileInfo.Size{
			logrus.Errorf("file size error!")
			return
		}

		fpath=strings.Split(fpath,",")[0]
		if err=s.util.WriteFileByOffSet(fpath,fileInfo.OffSet,data);err!=nil{
			logrus.Errorf("WriteFileByOffSet error!fpath=%s;err=%+v\n",fpath,err)
			return
		}
		//todo
		//s.SaveFileMd5Log(fileInfo,CONST_FILE_Md5_FILE_NAME)
	}
	if err=req.ToFile(fpathTmp);err!=nil{
		//todo
		//s.AppendToDownloadQueue(fileInfo);
		os.Remove(fpathTmp)
		logrus.Errorf("req.ToFile error!fpathTmp=%s;err=%+v;",fpathTmp,err)
		return
	}
	if fi.Size()!=fileInfo.Size{
		logrus.Errorf("file sum check error!")
		os.Remove(fpathTmp)
		return
	}

	if os.Rename(fpathTmp,fpath)==nil{
		//todo
		//s.SaveFileMd5Log(fileInfo,CONST_FILE_Md5_FILE_NAME);
	}
}

func (s *Server) CrossOrigin(w http.ResponseWriter,r *http.Request){
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Depth, User-Agent, X-File-Size, X-Requested-With, X-Requested-By, If-Modified-Since, X-File-Name, X-File-Type, Cache-Control, Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Expose-Headers", "Authorization")
	//https://blog.csdn.net/yanzisu_congcong/article/details/80552155
}

func (s *Server) SetDownloadHeader(w http.ResponseWriter,r *http.Request){
	w.Header().Set("Content-Type","application/octet-stream")
	w.Header().Set("Content-Disposition","attachment")
	if name,ok:=r.URL.Query()["name"];ok{
		if v,err:=url.QueryUnescape(name[0]);err==nil{
			name[0]=v
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s", name[0]))
	}
}


func (s *Server) CheckAuth(w http.ResponseWriter,r *http.Request) bool{
	var(
		err error
		req *httplib.BeegoHTTPRequest
		result string
		jsonResult JsonResult
	)
	if err=r.ParseForm();err!=nil{
		logrus.Errorf("ParseForm error!errr=%+v\n",err)
		return false
	}
	req=httplib.Post(Config().AuthUrl)
	req.SetTimeout(time.Second*10,time.Second*10)
	req.Param("__path__",r.URL.Path)
	req.Param("__query__",r.URL.RawQuery)
	for k,v:=range r.Form{
		logrus.Infof("form;k=%s;v=%s\n",k,v)
		req.Param(k,r.FormValue(k))
	}
	for k,v:=range r.Header{
		logrus.Infof("header;k=%s;v=%s\n",k,v)
		req.Header(k,v[0])
	}
	result,err=req.String()
	result=strings.TrimSpace(result)
	if strings.HasPrefix(result,"{")&&strings.HasSuffix(result,"}"){
		if err=json.Unmarshal([]byte(result),&jsonResult);err!=nil{
			logrus.Errorf("unmarshal error!err=%+v\n",err)
			return false
		}
		if jsonResult.Data!="ok"{
			logrus.Errorf("result not ok!")
			return false
		}
	}else if result!="ok"{
		logrus.Warnf("result not ok!")
		return false
	}
	return true
}

func (s *Server) NotPermit(w http.ResponseWriter,r *http.Request){
	w.WriteHeader(401)
}

func (s *Server) GetFilePathFromRequest(w http.ResponseWriter,r *http.Request) (string,string){
	var (
		err error
		fullPath string
		smallPath string
		prefix string
	)
	fullPath=r.RequestURI[1:]

	if strings.HasPrefix(r.RequestURI,"/"+Config().Group+"/"){
		fullPath=r.RequestURI[len(Config().Group)+2:len(r.RequestURI)]
	}
	fullPath=strings.Split(fullPath,"?")[0]//just path
	fullPath=DOCKER_DIR+STORE_DIR_NAME+"/"+fullPath
	prefix="/"+LARGE_DIR_NAME+"/"

	if Config().SupportGroupManage{
		prefix="/"+Config().Group+"/"+LARGE_DIR_NAME+"/"
	}
	if strings.HasPrefix(r.RequestURI,prefix){
		smallPath=fullPath
		fullPath=strings.Split(fullPath,",")[0]
	}
	if fullPath,err=url.PathUnescape(fullPath);err!=nil{
		logrus.Errorf("pathUnescape error!err=%+v\n",err)
	}
	return fullPath,smallPath
}

func (s *Server) CheckDownloadAuth(w http.ResponseWriter,r *http.Request) (bool,error){
	var(
		err error
		maxTimestamp int64
		minTimestamp int64
		ts int64
		token string
		timestamp string
		fullPath string
		smallPath string
		pathMd5 string
		fileInfo *FileInfo
		scene string
		secret interface{}
		code string
		ok bool
	)
	CheckToken:=func(token string,md5sum string ,timestamp string) bool{
		if s.util.MD5(md5sum+timestamp)!=token{
			return false
		}else{
			return true
		}
	}

	//todo
	//&&!s.IsPeer(r)&&!s.CheckAuth(w,r)
	if Config().EnableDownloadAuth&&Config().AuthUrl!=""{
		return false,errors.New("auth fail!")
	}
	//todo
	//&& s.IsPeer(r)
	if Config().DownloadUseToken{
		token=r.FormValue("token")
		timestamp=r.FormValue("timestamp")

		if token==""||timestamp==""{
			return false,errors.New("invalid request!need token and timestamp!")
		}

		maxTimestamp=time.Now().Add(time.Second*time.Duration(Config().DownloadTokenExpire)).Unix()
		minTimestamp=time.Now().Add(-time.Second*time.Duration(Config().DownloadTokenExpire)).Unix()
		if ts,err=strconv.ParseInt(timestamp,10,64);err!=nil{
			return false, errors.New(fmt.Sprintf("invalid timestamp!timestamp=%+v\n",timestamp))
		}
		if ts<minTimestamp||ts>maxTimestamp{
			return false,errors.New(fmt.Sprintf("timestamp expire!ts=%d",ts))
		}
		fullPath,smallPath=s.GetFilePathFromRequest(w,r)
		if smallPath!=""{
			pathMd5=s.util.MD5(smallPath)
		}else {
			pathMd5=s.util.MD5(fullPath)
		}

		//todo
/*		if fileInfo,err=s.GetFileInfoFromLevelDB(pathMd5);err!=nil{
			//todo
		}else {
			ok:=CheckToken(token,fileInfo.Md5,timestamp)
			if !ok{
				return ok,errors.New(fmt.Sprintf("invalid token!token=%s\n",token))
			}
			return ok,nil
		}*/
	}
	//todo
	//s.IsPeer(r)
	if Config().EnableGoogleAuth{
		fullPath=r.RequestURI[len(Config().Group)+2:]
		fullPath=strings.Split(fullPath,"?")[0]
		scene=strings.Split(fullPath,"/")[0]
		code=r.FormValue("code")

		if secret,ok=s.sceneMap.GetValue(scene);ok{
			//todo
/*			if !s.VerifyGoogleCode(secret.(string),code,int64(Config().DownloadTokenExpire/30)){
				return false,errors.New(fmt.Sprintf("invalid google code!scene=%+v;secret=%+v;code=%+v;",scene,secret,code))
			}*/
		}

	}
	return true,nil
}


func (s *Server) GetSmallFileByURI(w http.ResponseWriter,r *http.Request)([]byte,bool,error){
	var (
		err error
		data []byte
		offset int64
		length int
		fullPath string
		info os.FileInfo
	)
	fullPath,_=s.GetFilePathFromRequest(w,r)

	if _,offset,length,err=s.ParseSmallFile(r.RequestURI);err!=nil{
		return nil, false, err
	}
	if info,err=os.Stat(fullPath);err!=nil{
		return nil, false, err
	}
	if info.Size()<(offset+int64(length)){
		return nil,false,errors.New(fmt.Sprintf("no found!"))
	}else {
		if data,err=s.util.ReadFileByOffSet(fullPath,offset,length);err!=nil{
			return nil, false, err
		}
		return data,false,err
	}

}

func (s *Server) DownloadSmallFileByURI(w http.ResponseWriter,r *http.Request) (bool,error){
	var (
		err error
		data []byte
		isDownload bool
		imgWidth int
		imgHeight int
		width ,height string
		notFound bool
	)
	r.ParseForm()
	isDownload=true
	if r.FormValue("download")==""{
		isDownload=Config().DefaultDownload
	}
	if r.FormValue("download")=="0"{
		isDownload=false
	}
	width=r.FormValue("width")
	height=r.FormValue("height")
	if imgWidth,err=strconv.Atoi(width);err!=nil{
		logrus.Errorf("width error!width=%s",width)
	}
	if imgHeight,err=strconv.Atoi(height);err!=nil{
		logrus.Errorf("height error!height=%s",height)
	}
	data,notFound,err=s.GetSmallFileByURI(w,r)
	if data!=nil&&data[0]==1{
		if isDownload{
			s.SetDownloadHeader(w,r)
		}
		if imgWidth!=0||imgHeight!=0{
			//todo
			//s.ResizeImageByBytes(w,data[1:],uint(imgWidth),uint(imgHeight))
			return true,nil
		}
		w.Write(data[1:])
		return true,nil
	}
	return false,errors.New("not found!")
}

func (s *Server) DownloadNormalFileByURI(w http.ResponseWriter,r *http.Request) (bool,error){
	var(
		err error
		isDownload bool
		imgWidth,imgHeight int
		width,height string
	)
	r.ParseForm()
	isDownload=true
	downloadStr:=r.FormValue("download")
	if downloadStr==""{
		isDownload=Config().DefaultDownload
	}else if downloadStr=="0"{
		isDownload=false
	}
	width=r.FormValue("width")
	height=r.FormValue("height")
	if imgWidth,err=strconv.Atoi(width);err!=nil{
		logrus.Errorf("width error!width=%s;err=%+v",width,err)
	}
	if imgHeight,err=strconv.Atoi(height);err!=nil{
		logrus.Errorf("width error!height=%s;err=%+v",height,err)
	}

	if isDownload{
		s.SetDownloadHeader(w,r)
	}

	fullPath,_:=s.GetFilePathFromRequest(w,r)
	if imgWidth!=0||imgHeight!=0{
		//todo
		//s.ResizeImage(w,fullPath,uint(imgWidth),uint(imgHeight))
		return true,nil
	}
	staticHandler.ServeHTTP(w,r)
	return true,nil
}

func (s *Server) DownloadNotFound(w http.ResponseWriter,r *http.Request){
	var (
		err error
		smallPath,fullPath string
		isDownload bool
		pathMd5,peer string
		fileInfo *FileInfo
	)
	fullPath,smallPath=s.GetFilePathFromRequest(w,r)
	isDownload=true
	downloadStr:=r.FormValue("download")
	if downloadStr==""{
		isDownload=Config().DefaultDownload
	}else if downloadStr=="0"{
		isDownload=false
	}

	if smallPath!=""{
		pathMd5=s.util.MD5(smallPath)
	}else {
		pathMd5=s.util.MD5(fullPath)
	}
	//todo
/*	for _,peer = range Config().Peers{
		if fileInfo,err=s.checkPeerFileExist(peer,pathMd5,fullPath);err!=nil{
			logrus.Errorf("checkPeerFileExists error!err=%+v",err)
			continue
		}
		if fileInfo.Md5!=""{
			go s.DownloadFromPeer(peer,fileInfo)

			if isDownload{
				s.SetDownloadHeader(w,r)
			}
			s.DownloadFileToResponse(peer+r.RequestURI,w,r)
			return
		}
	}*/
    w.WriteHeader(404)
	return
}

func (s *Server) Download(w http.ResponseWriter,r *http.Request){
	var (
		err error
		ok bool
		smallPath,fullPath string
		fi os.FileInfo
	)
	//redirect to upload
	if r.RequestURI=="/"||r.RequestURI==""||r.RequestURI=="/"+Config().Group||r.RequestURI=="/"+Config().Group+"/"{
		//todo
		//s.Index(w,r)
		return
	}

	if ok,err=s.CheckDownloadAuth(w,r);!ok{
		log.Errorf("CheckDownloadAuth error!err=%+v\n",err)
		s.NotPermit(w,r)
		return
	}
	if Config().EnableCrossOrigin{
		s.CrossOrigin(w,r)
	}
	fullPath,smallPath=s.GetFilePathFromRequest(w,r)
	if smallPath==""{
		if fi,err=os.Stat(fullPath);err!=nil{
			s.DownloadNotFound(w,r)
			return
		}
		if !Config().ShowDir&&fi.IsDir(){
			w.Write([]byte("list dir deny"))
			return
		}
		s.DownloadNormalFileByURI(w,r)
		return
	}
	if smallPath!=""{
		if ok,err=s.DownloadSmallFileByURI(w,r);!ok{
			s.DownloadNotFound(w,r)
			return
		}
		return
	}
}

func (s *Server) DownloadFileToResponse(url string,w http.ResponseWriter,r *http.Request){
	var (
		err error
		req *httplib.BeegoHTTPRequest
		resp *http.Response
	)
	req=httplib.Get(url)
	req.SetTimeout(time.Second*20,time.Second*600)
	if resp,err=req.DoRequest();err!=nil{
		logrus.Errorf("DoRequest error!err=%+v",err)
		return
	}
	defer resp.Body.Close()
	if _,err=io.Copy(w,resp.Body);err!=nil{
		logrus.Errorf("Copy error!err=%+v\n",err)
	}
}

func (s *Server) ResizeImageByBytes(w http.ResponseWriter,data []byte,width,height uint){
	var(
		img image.Image
		err error
		imgType string
	)
	reader:=bytes.NewReader(data)
	if img,imgType,err=image.Decode(reader);err!=nil{
		logrus.Errorf("decode error!err=%+v",err)
		return
	}
	img=resize.Resize(width,height,img,resize.Lanczos3)
	if imgType=="jpg"||imgType=="jpeg"{
		jpeg.Encode(w,img,nil)
	}else if imgType=="png"{
		png.Encode(w,img)
	}else{
		w.Write(data)
	}
}

func (s *Server) ResizeImage(w http.ResponseWriter,fullPath string,width,height uint){
	var (
		img image.Image
		err error
		imgType string
		file *os.File
	)
	if file,err=os.Open(fullPath);err!=nil{
		logrus.Errorf("open file error!fullPath=%s;err=%+v;",fullPath,err)
		return
	}
	if img,imgType,err=image.Decode(file);err!=nil{
		logrus.Errorf("image decode error!err=%+v",err)
		return
	}
	file.Close()

	img=resize.Resize(width,height,img,resize.Lanczos3)
	if imgType=="jpg"||imgType=="jpeg"{
		jpeg.Encode(w,img,nil)
	}else if imgType=="png"{
		png.Encode(w,img)
	}else {
		file.Seek(0,0)
		io.Copy(w,file)
	}
}

func (s *Server) GetServerURI(r *http.Request) string{
	return fmt.Sprintf("http://%s/",r.Host)
}

func (s *Server) CheckFileAndSendToPeer(date string,fileName string,isForceUpload bool){
	logrus.Infof("date=%+v;fileName=%+v;isForceUpload=%+v;",date,fileName,isForceUpload)
	var (
		md5set mapset.Set
		err error
		md5s []interface{}
		fileInfo *FileInfo
	)
	defer func(){
		if re:=recover();re!=nil{
			buffer:=debug.Stack()
			logrus.Errorf("CheckFileAndSendToPeer;re=%+v;buffer=%+v;",re,string(buffer))
		}
	}()
	//todo
/*	if md5set,err=s.GetMd5sByDate(date,fileName);err!=nil{
		log.Errorf("GetMd5sByDate error!date=%s;fileName=%s;err=%+v;",date,fileName,err)
		return
	}*/
	md5s=md5set.ToSlice()
	for _,md5:=range md5s{
		if md5==nil{
			continue
		}
/*		if fileInfo,_=s.GetFileInfoFromLevelDB(md5.(string));fileInfo!=nil&&fileInfo.Md5!=""{
			if isForceUpload{
				fileInfo.Peers=[]string{}
			}
			if len(fileInfo.Peers)>len(Config().Peers){
				continue
			}

			if s.util.Contains(s.host,fileInfo.Peers){
				fileInfo.Peers=append(fileInfo.Peers,s.host)
			}
			if fileName==CONST_Md5_QUEUE_FILE_NAME{
				//todo
				//s.AppendToDownloadQueue(fileInfo)
			}else {
				//todo
				//s.AppendToQueue(fileInfo)
			}
		}*/

	}
}


func(s *Server) postFileToPeer(fileInfo *FileInfo){
	var (
		err error
		peer,fileName,postURL,result,fpath string
		info *FileInfo
		fi os.FileInfo
		i int
		data []byte
	)
	defer func(){
		if re:=recover();re!=nil{
			buffer:=debug.Stack()
			logrus.Errorf("postFileToPeer;re=%+v;buffer=%+v;",re,string(buffer))
		}
	}()

	for i,peer=range Config().Peers{
		if fileInfo.Peers==nil{
			fileInfo.Peers=[]string{}
		}
		if s.util.Contains(peer,fileInfo.Peers){
			continue
		}
		fileName=fileInfo.Name
		if fileInfo.ReName!=""{
			fileName=fileInfo.ReName
			if fileInfo.OffSet!=-1{
				fileName=strings.Split(fileInfo.ReName,",")[0]
			}
		}
		fpath=DOCKER_DIR+fileInfo.Path+"/"+fileName
		if !s.util.FileExists(fpath){
			logrus.Warnf("file %s not found!",fpath)
			continue
		}else {
		    if fileInfo.Size==0{
		    	if fi,err=os.Stat(fpath);err!=nil{
		    		logrus.Errorf("os.Stat error!fpath=%s;err=%s;",fpath,err)
				}else {
					fileInfo.Size=fi.Size()
				}
			}
		}

		//todo
/*		if fileInfo.OffSet!=-2&&Config().EnableDistinctFile{
			if info,err=s.CheckPeerFileExist(peer,fileInfo.Md5,"");info.Md5!=""{
				fileInfo.Peers=append(fileInfo.Peers,peer)
				if _,err=s.SaveFileInfoToLevelDB(fileInfo.Md5,fileInfo,s.ldb);err!=nil{
					logrus.Errorf("SaveFileInfoToLevelDB error!err=%+v;",err)
				}
				continue
			}
		}*/
		//todo
        //postURL=fmt.Sprintf("%s%s",peer,s.getRequestURI("syncfile_info"))
        b:=httplib.Post(postURL)
        b.SetTimeout(time.Second*30,time.Second*30)
        if data,err=json.Marshal(fileInfo);err!=nil{
        	logrus.Errorf("marshal error!err=%+v\n",err)
			return
		}
		b.Param("fileInfo",string(data))
        result,err=b.String()
        if err!=nil{
        	if fileInfo.retry<=Config().RetryCount{
        		fileInfo.retry=fileInfo.retry+1
        		//todo
        		//s.AppendToQueue(fileInfo)
			}
			logrus.Errorf("http requet error!err=%%+v;path=%s;",err, fileInfo.Path+"/"+fileInfo.Name)
		}
		if !strings.HasPrefix(result,"http://"){
			logrus.Infof("result=%s;",result)
			if !s.util.Contains(peer,fileInfo.Peers){
				fileInfo.Peers=append(fileInfo.Peers,peer)
				//todo
	/*			if _,err=s.SaveFileInfoToLevelDB(fileInfo.Md5,fileInfo,s.ldb);err!=nil{
					logrus.Errorf("SaveFileInfoToLevelDB error!err=%+v",err)
				}*/
			}
		}

		if err!=nil{
			logrus.Errorf("err=%+v",err)
		}

	}//for
}

func (s *Server) SaveFileMd5Log(fileInfo *FileInfo,fileName string){
	for len(s.queueFileLog)+len(s.queueFileLog)/10>CONST_QUEUE_SIZE{
		time.Sleep(time.Second*1)
	}
	info:=*fileInfo
	s.queueFileLog<-&FileLog{FileInfo:&info,FileName:fileName}
}

func (s *Server) saveFileMd5Log(fileInfo *FileInfo,fileName string){
	var(
		err error
		outname,logDate,fullPath,md5Path,logKey string
		ok bool
	)
	defer func(){
		if re:=recover();re!=nil{
			buffer:=debug.Stack()
			logrus.Errorf("saveFileMd5Log;re=%+v;buffer=%+v;",re,string(buffer))
		}
	}()
	if fileInfo==nil||fileInfo.Md5==""||fileName==""{
		logrus.Warnf("saveFileMd5Log!fileInfo=%+v;fileName=%s;",fileInfo,fileName)
		return
	}
	logDate=s.util.GetDayFromTimeStamp(fileInfo.TimeStamp)
	outname=fileInfo.Name
	if fileInfo.ReName!=""{
		outname=fileInfo.ReName
	}
	fullPath=fileInfo.Path+"/"+outname
	logKey=fmt.Sprintf("%s_%s_%s",logDate,fileName,fileInfo.Md5)
	if fileName==CONST_FILE_Md5_FILE_NAME{
		//todo
/*		if ok,err=s.IsExistFromLevelDB(fileInfo.Md5,s.ldb);!ok{
			s.statMap.AddCountInt64(logDate+"_"+CONST_STAT_FILE_COUNT_KEY,1)
			s.statMap.AddCountInt64(logDate+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY,fileInfo.Size)
			s.SaveStat()
		}*/

		//todo
/*		if _,err=s.SaveFileInfoToLevelDB(logKey,fileInfo,s.logDB);err!=nil{
		   logrus.Errorf("SaveFileInfoToLevelDB error!logKey=%s;err=%+v",logKey,err)
		}

		if _,err=s.SaveFileInfoToLevelDB(fileInfo.Md5,fileInfo,s.ldb);err!=nil{
			logrus.Errorf("SaveFileInfoToLevelDB error!fileInfo.Md5=%s;err=%+v",fileInfo.Md5,err)
		}

		if _,err=s.SaveFileInfoToLevelDB(s.util.MD5(fullPath),fileInfo,s.ldb);err!=nil{
			logrus.Errorf("SaveFileInfoToLevelDB error!fullPath=%s;err=%+v",fullPath,err)
		}*/
		return
	}

/*	if fileName==CONST_REMOVE_Md5_FILE_NAME{
		//todo
		if ok,err=s.IsExistFromLevelDB(fileInfo.Md5,s.ldb);ok{
			s.statMap.AddCountInt64(logDate+"_"+CONST_STAT_FILE_COUNT_KEY,-1)
			s.statMap.AddCountInt64(logDate+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY,-fileInfo.Size)
			s.SaveStat()
		}
		s.RemoveKeyFromLevelDB(logKey,s.ldb)

		md5Path=s.util.MD5(fullPath)
		if err:=s.RemoveKeyFromLevelDB(fileInfo.Md5,s.ldb);err!=nil{
			logrus.Errorf("RemoveKeyFromLevelDB error!fileInfo.Md5=%s;err=%+v;",fileInfo.Md5,err)
		}
		if err:=s.RemoveKeyFromLevelDB(md5Path,s.ldb);err!=nil{
			logrus.Errorf("RemoveKeyFromLevelDB error!md5Path=%s;err=%+v;",md5Path,err)
		}
		logKey=fmt.Sprintf("%s_%s_%s",logDate,CONST_FILE_Md5_FILE_NAME,fileInfo.Md5)
		s.RemoveKeyFromLevelDB(logKey,s.logDB)
		return
	}*/
	//todo
	//s.SaveFileInfoToLevelDB(logKey,fileInfo,s.logDB)
}

func (s *Server) checkPeerFileExist(peer string,md5sum string,fpath string) (*FileInfo,error){
	var (
		err error
		fileInfo FileInfo
	)
	//todo
	req:=httplib.Post(fmt.Sprintf("%s%s?md5=%s",peer,s.getRequestURI("check_file_exist"),md5sum))
	req.Param("path",fpath)
	req.Param("md5",md5sum)
	req.SetTimeout(time.Second*5,time.Second*10)

	if err=req.ToJSON(&fileInfo);err!=nil{
		return &FileInfo{},err
	}

	if fileInfo.Md5==""{
		return &fileInfo,errors.New("not found")
	}
	return &fileInfo,nil
}

func (s *Server) CheckFileExist(w http.ResponseWriter,r *http.Request){
	var (
		data []byte
		err error
		fileInfo *FileInfo
		fpath string
		fi os.FileInfo
	)
	r.ParseForm()
	md5sum:=r.FormValue("md5")
	fpath=r.FormValue("path")
	//todo
	if fileInfo,err=s.GetFileInfoFromLevelDB(md5sum);fileInfo!=nil{
		if fileInfo.OffSet!=-1{
			if data,err=json.Marshal(fileInfo);err!=nil{
				logrus.Errorf("marshal error!fileInfo=%+v;err=%+v;",fileInfo,err)
			}
			w.Write(data)
			return
		}
		fpath=DOCKER_DIR+fileInfo.Path+"/"+fileInfo.Name
		if fileInfo.ReName!=""{
			fpath=DOCKER_DIR+fileInfo.Path+"/"+fileInfo.ReName
		}
		if s.util.IsExist(fpath){
			if data,err=json.Marshal(fileInfo);err==nil{
				w.Write(data)
				return
			}else {
				logrus.Errorf("Marshal error!fileInfo=%+v;err=%+v;",fileInfo,err)
			}
		}else {
			if fileInfo.OffSet==-1{
				//todo
				//s.RemoveKeyFromLevelDB(md5sum,s.ldb)
			}
		}
	}else {
		if fpath!=""{
			if fi,err=os.Stat(fpath);err==nil{
				sum:=s.util.MD5(fpath)
				fileInfo=&FileInfo{
					Path:path.Dir(fpath),
					Name: path.Base(fpath),
					Size: fi.Size(),
					Md5:sum,
					Peers: []string{Config().Host},
					OffSet: -1,
					TimeStamp: fi.ModTime().Unix(),
				}
			}
			data,err=json.Marshal(fileInfo)
			w.Write(data)
			return
		}
	}
	data,_=json.Marshal(FileInfo{})
	w.Write(data)
}

func (s *Server) Sync(w http.ResponseWriter,r *http.Request){
	r.ParseForm()
	var jsonResult JsonResult
	jsonResult.Status="fail"
	//todo
	if !s.IsPeer(r){
		jsonResult.Message="client must be in cluster"
		w.Write([]byte(s.util.JsonEncodePretty(jsonResult)))
		return
	}
	date:=""
	force:=""
	inner:=""
	isForceUpload:=false
	force=r.FormValue("force")
	date=r.FormValue("date")
	inner=r.FormValue("inner")

	if force=="1"{
		isForceUpload=true
	}
	if inner!="1"{
		for _,peer:=range Config().Peers{
			req:=httplib.Post(peer+s.getRequestURI("sync"))
			req.Param("force",force)
			req.Param("inner",inner)
			req.Param("date",date)
			if _,err:=req.String();err!=nil{
				logrus.Errorf("req error!err=%+v",err)
			}
		}//for
	}
	if date == ""{
		jsonResult.Message="require params date &force,?date=20181230"
		w.Write([]byte(s.util.JsonEncodePretty(jsonResult)))
		return
	}
	date=strings.Replace(date,".","",-1)

	if isForceUpload{
		go s.CheckFileAndSendToPeer(date,CONST_FILE_Md5_FILE_NAME,isForceUpload)
	}else {
		go s.CheckFileAndSendToPeer(date,CONST_Md5_ERROR_FILE_NAME,isForceUpload)
	}
	jsonResult.Status="ok"
	jsonResult.Message="jos is running"
	w.Write([]byte(s.util.JsonEncodePretty(jsonResult)))
}

func (s *Server) IsExistFromLevelDB(key string,db *leveldb.DB) (bool,error){
	return db.Has([]byte(key),nil)
}

func (s *Server) GetFileInfoFromLevelDB(key string) (*FileInfo,error){
	var (
		err error
		data []byte
		fileInfo FileInfo
	)
	if data,err=s.ldb.Get([]byte(key),nil);err!=nil{
		return nil, err
	}
	if err=json.Unmarshal(data,&fileInfo);err!=nil{
		return nil, err
	}
	return &fileInfo,nil
}

func (s *Server) SaveStat(){
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("SaveStatFunc")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	stat:=s.statMap.Get()
	if v,ok:=stat[CONST_STAT_FILE_COUNT_KEY];ok{
		switch v.(type) {
		case int64,int32,int,float64,float32:
			if v.(int64)>=0{
				if data,err:=json.Marshal(stat);err!=nil{
					logrus.Errorf("Marshal error!stat=%+v;err=%+v;",stat,err)
				}else {
					s.util.WriteBinFile(CONST_STAT_FILE_NAME,data)
				}
			}
		}
	}

}

func (s *Server) RemoveKeyFromLevelDB(key string,db *leveldb.DB) error{
	return  db.Delete([]byte(key),nil)
}

func (s *Server) SaveFileInfoToLevelDB(key string,fileInfo* FileInfo,db *leveldb.DB) (*FileInfo,error){
	if fileInfo==nil||db==nil{
		return nil,errors.New("fileInfo is null or db is null")
	}
	var (
		data []byte
		err error
	)
	if data,err=json.Marshal(fileInfo);err!=nil{
		return fileInfo, err
	}
	if err=db.Put([]byte(key),data,nil);err!=nil{
		return fileInfo,err
	}
	if db==s.ldb{
		logDate:=s.util.GetDayFromTimeStamp(fileInfo.TimeStamp)
		logKey:=fmt.Sprintf("%s_%s_%s",logDate,CONST_FILE_Md5_FILE_NAME,fileInfo.Md5)
		s.logDB.Put([]byte(logKey),data,nil)
	}
	return fileInfo,nil
}

func isPublicIP(IP *net.IP) bool{
	if IP.IsLoopback()||IP.IsLinkLocalMulticast()||IP.IsLinkLocalUnicast(){
		return false
	}
	if ip4:=IP.To4();ip4!=nil{
		switch true{
		case ip4[0]==10||(ip4[0]==172&&ip4[1]>=16&&ip4[1]<=31)||(ip4[0]==192&&ip4[1]==168):
			return false
		default:
			return true
		}
	}
	return false
}

func (s *Server) IsPeer(r *http.Request) bool{
	var (
		ip string
		peer string
		bflag bool
		cidr *net.IPNet
		err error
	)
	ip=s.util.GetClientIp(r)
	clientIP:=net.ParseIP(ip)
	if s.util.Contains("0.0.0.0",Config().AdminIps){
		if isPublicIP(&clientIP){
			return false
		}
		return true
	}

	if s.util.Contains(ip,Config().AdminIps){
		return true
	}
	for _,v:=range Config().AdminIps{
		if strings.Contains(v,"/"){
		   if _,cidr,err=net.ParseCIDR(v);err!=nil{
		   	  logrus.Errorf("ParseCIDR error!v=%s;err=%+v",v,err)
			   return false
		   }
		   if cidr.Contains(clientIP){
		   	return true
		   }
		}
	}//for
	realIp:=os.Getenv("GO_FASTDFS_IP")
	if realIp==""{
		realIp=s.util.GetPulicIP()
	}
	if ip=="127.0.0.1"||ip==realIp{
		return true
	}
	ip="http://"+ip
	bflag=false
	for _,peer=range Config().Peers{
		if strings.HasPrefix(peer,ip){
			bflag=true
			break
		}
	}
	return bflag
}

func (s *Server) RecvMd5s(w http.ResponseWriter,r *http.Request){
	var (
		err error
		md5str string
		fileInfo *FileInfo
		md5s []string
	)
	if !s.IsPeer(r){
		logrus.Warnf("RecvMd5s;ip=%s;",s.util.GetClientIp(r))
		//todo
		//w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}
	r.ParseForm()

	md5str=r.FormValue("md5s")
	md5s=strings.Split(md5str,",")
	go func(md5s []string){
		for _,m:=range md5s{
			if m!=""{
				if fileInfo,err=s.GetFileInfoFromLevelDB(m);err!=nil{
					logrus.Errorf("GetFileInfoFromLevelDB error!m=%s;err=%+v;",m,err)
					continue
				}
				//todo
				//s.AppendToQueue(fileInfo)
			}
		}
	}(md5s)
}

func (s *Server) GetClusterNotPermitMessage(r *http.Request) string{
	return fmt.Sprintf(CONST_MESSAGE_CLUSTER_IP,s.util.GetClientIp(r))
}

func (s *Server) GetMd5sForWeb(w http.ResponseWriter,r *http.Request){
	var (
		date string
		err error
		result mapset.Set
		lines []string
		md5s []interface{}
	)
	if !s.IsPeer(r){
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}
	date=r.FormValue("date")
	//todo
	if result,err=s.GetMd5sByDate(date,CONST_FILE_Md5_FILE_NAME);err!=nil{
		logrus.Errorf("GetMd5sByDate error!date=%s;err=%+v;",date,err)
		return
	}

	md5s=result.ToSlice()
	for _,line:=range md5s{
		if line!=nil&&line!=""{
			lines=append(lines,line.(string))
		}
	}
	w.Write([]byte(strings.Join(lines,",")))
}

func (s *Server) GetMd5File(w http.ResponseWriter,r *http.Request){
	var(
		date string
		fpath string
		data []byte
		err error
	)
	if !s.IsPeer(r){
		return
	}
	fpath=DATA_DIR+"/"+date+"/"+CONST_FILE_Md5_FILE_NAME

	if !s.util.FileExists(fpath){
		w.WriteHeader(404)
		return
	}
	if data,err=ioutil.ReadFile(fpath);err!=nil{
		w.WriteHeader(500)
		return
	}
	w.Write(data)
}

func (s *Server) GetMd5sMapByDate(date ,fileName string) (*goutil.CommonMap,error){
	var (
		err error
		result *goutil.CommonMap
		fpath,content,line string
		lines,cols []string
		data []byte
	)
	result=goutil.NewCommonMap(0)
	if fileName==""{
		fpath=DATA_DIR+"/"+date+"/"+CONST_FILE_Md5_FILE_NAME
	}else {
		fpath=DATA_DIR+"/"+date+"/"+fileName
	}
	if !s.util.FileExists(fpath){
		return result,errors.New(fmt.Sprintf("fpath %s not found!",fpath))
	}
	if data,err=ioutil.ReadFile(fpath);err!=nil{
		return result, err
	}
	content=string(data)
	lines=strings.Split(content,"\n")
	for _,line=range lines{
		cols=strings.Split(line,"|")
		if len(cols)>2{
			if _,err=strconv.ParseInt(cols[1],10,64);err!=nil{
				continue
			}
			result.Add(cols[0])
		}
	}//for
	return result,nil
}

func (s *Server) GetMd5sByDate(date string,fileName string) (mapset.Set,error){
	var (
		keyPrefix string
		md5set mapset.Set
		keys []string
	)
	md5set=mapset.NewSet()
	keyPrefix="%s_%s_"
	keyPrefix=fmt.Sprintf("%s_%s_",date,fileName)
	it:=server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)),nil)
	for it.Next(){
		keys=strings.Split(string(it.Key()),"_")
		if len(keys)>=3{
			md5set.Add(keys[2])
		}
	}//for
	it.Release()
	return md5set,nil
}

func (s *Server) SyncFileInfo(w http.ResponseWriter,r *http.Request){
	var (
		err error
		fileInfo FileInfo
		fileInfoStr string
		fileName string
	)
	r.ParseForm()
	fileInfoStr=r.FormValue("fileInfo")

	if !s.IsPeer(r){
		logrus.Errorf("is not peer fileInfo!fileInfo=%+v",fileInfo)
		return
	}
	if err=json.Unmarshal([]byte(fileInfoStr),&fileInfo);err!=nil{
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		logrus.Errorf("unmarshal error!err=%+v",err)
		return
	}
	if fileInfo.OffSet==-2{
		s.SaveFileInfoToLevelDB(fileInfo.Md5,&fileInfo,s.ldb)
	}else {
		s.SaveFileMd5Log(&fileInfo,CONST_Md5_QUEUE_FILE_NAME)
	}
	//todo
	s.AppendToDownloadQueue(&fileInfo)
	fileName=fileInfo.Name

	if fileInfo.ReName!=""{
		fileName=fileInfo.ReName
	}

	p:=strings.Replace(fileInfo.Path,STORE_DIR+"/","",1)
	downloadUrl:=fmt.Sprintf("http://%s/%s",r.Host,Config().Group+"/"+p+"/"+fileName)
	logrus.Infof("SyncFileInfo!downloadUrl=%s;",downloadUrl)
	w.Write([]byte(downloadUrl))
}


func (s *Server) CheckScene(scene string) (bool,error){

	if len(Config().Scenes)==0{
		return true,nil
	}
	var scenes []string
	for _,s:=range Config().Scenes{
		scenes=append(scenes,strings.Split(s,":")[0])
	}
	if !s.util.Contains(scene,scenes){
		return false,errors.New("not valid scene")
	}
	return true,nil
}

func (s *Server) GetFileInfo(w http.ResponseWriter,r *http.Request){
	var(
		fpath string
		md5sum string
		fileInfo *FileInfo
		err error
		result JsonResult
	)
	md5sum=r.FormValue("md5")
	fpath=r.FormValue("path")
	result.Status="fail"

	if !s.IsPeer(r){
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}

	md5sum=r.FormValue("md5")
	if fpath!=""{
		fpath=strings.Replace(fpath,"/"+Config().Group+"/",STORE_DIR_NAME+"/",1)
		md5sum=s.util.MD5(fpath)
	}

	if fileInfo,err =s.GetFileInfoFromLevelDB(md5sum);err!=nil{
		logrus.Errorf("GetFileInfoFromLevelDB error!err=%+v",err)
		result.Message=err.Error()
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}

	result.Status="ok"
	result.Data=fileInfo
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) RemoveFile(w http.ResponseWriter,r *http.Request){
	var (
		err error
		md5sum,fpath,delUrl,inner,name string
		fileInfo *FileInfo
		result JsonResult
	)
	r.ParseForm()
	md5sum=r.FormValue("md5")
	fpath=r.FormValue("path")
	inner=r.FormValue("inner")
	result.Status="fail"
	if !s.IsPeer(r){
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}
	if Config().AuthUrl!=""&&!s.CheckAuth(w,r){
		s.NotPermit(w,r)
		return
	}
    if fpath!=""&&md5sum==""{
    	fpath=strings.Replace(fpath,"/"+Config().Group+"/",STORE_DIR_NAME+"/",1)
    	md5sum=s.util.MD5(fpath)
	}
	if inner!="1"{
		for _,peer:=range Config().Peers{
			go func(peer string,md5sum string,fileInfo *FileInfo){
				//todo
				delUrl=fmt.Sprintf("%s%s",peer,s.getRequestURI("delete"))
				req:=httplib.Post(delUrl)
				req.Param("md5",md5sum)
				req.Param("inner","1")
				req.SetTimeout(time.Second*5,time.Second*10)
				if _,err=req.String();err!=nil{
				    logrus.Errorf("req error!err=%+v",err)
				}
			}(peer,md5sum,fileInfo)
		}//for
	}

	if len(md5sum)<32{
		result.Message="md5 unvalid"
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
    if fileInfo,err=s.GetFileInfoFromLevelDB(md5sum);err!=nil{
    	result.Message=err.Error()
    	w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}

	if fileInfo.OffSet>=0{
		result.Message="small file delete not support!"
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	name=fileInfo.Name
	if fileInfo.ReName!=""{
		name=fileInfo.ReName
	}
	fpath=fileInfo.Path+"/"+name
	if fileInfo.Path!=""&&s.util.FileExists(DOCKER_DIR+fpath){
		s.SaveFileMd5Log(fileInfo,CONST_REMOVE_Md5_FILE_NAME)
		if err=os.Remove(DOCKER_DIR+fpath);err!=nil{
			result.Message=err.Error()
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}else {
			result.Message="remove success"
			result.Status="ok"
			w.Write([]byte(s.util.JsonEncodePretty(result)))
		}
	}
	result.Message="fail to remove"
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) getRequestURI(action string) string{
	var uri string
	if Config().SupportGroupManage{
		uri="/"+Config().Group+"/"+action
	}else {
		uri="/"+action
	}
	return uri
}

func (s *Server) BuildFileResult(fileInfo *FileInfo,r *http.Request) FileResult{
	var (
		outname string
		fileResult FileResult
		p string
		downloadUrl string
		domain string
		host string
		protocol string
	)
	if Config().EnableHttps{
		protocol="https"
	}else {
		protocol="http"
	}
	host=strings.Replace(Config().Host,"http://","",-1)
	if r!=nil{
		host=r.Host
	}
	if !strings.HasPrefix(Config().DownloadDomain,"http"){
		if Config().DownloadDomain==""{
			Config().DownloadDomain=fmt.Sprintf("%s://%s",protocol,host)
		}else {
			Config().DownloadDomain=fmt.Sprintf("%s://%s",protocol,Config().DownloadDomain)
		}
	}

	if Config().DownloadDomain!=""{
		domain=Config().DownloadDomain
	}else {
		domain=fmt.Sprintf("%s://%s",protocol,host)
	}
	outname=fileInfo.Name
	if fileInfo.ReName!=""{
		outname=fileInfo.ReName
	}
	p=strings.Replace(fileInfo.Path,STORE_DIR_NAME+"/","",1)
	if Config().SupportGroupManage{
		p=Config().Group+"/"+p+"/"+outname
	}else {
		p=p+"/"+outname
	}
	downloadUrl=fmt.Sprintf("%s://%s/%s",protocol,host,p)

	if Config().DownloadDomain!=""{
		downloadUrl=fmt.Sprintf("%s/%s",Config().DownloadDomain,p)
	}
	fileResult.Url=downloadUrl
	if Config().DefaultDownload{
		fileResult.Url=fmt.Sprintf("%s?name=%s&download=1",downloadUrl,url.PathEscape(outname))
	}
	fileResult.Md5=fileInfo.Md5
	fileResult.Path="/"+p
	fileResult.Domain=domain
	fileResult.Scene=fileInfo.Scene
	fileResult.Size=fileInfo.Size
	fileResult.ModTime=fileInfo.TimeStamp
	fileResult.Src=fileResult.Path
	fileResult.Scenes=fileInfo.Scene

	return fileResult
}

func (s *Server) SaveUploadFile(file multipart.File,header *multipart.FileHeader,
	fileInfo *FileInfo,r *http.Request) (*FileInfo,error){
	var(
		err error
		outFile *os.File
		folder string
		fi os.FileInfo
	)
	defer file.Close()
	_,fileInfo.Name=filepath.Split(header.Filename)
	if len(Config().Extensions)>0&&!s.util.Contains(path.Ext(fileInfo.Name),Config().Extensions){
		return fileInfo,errors.New("error file extension mismatch!")
	}
	if Config().RenameFile{
		fileInfo.ReName=s.util.MD5(s.util.GetUUID()+path.Ext(fileInfo.Name))
	}
	folder=time.Now().Format("20060102/15/04")
	if Config().PeerId!=""{
		folder=fmt.Sprintf("folder/%s",Config().PeerId)
	}else {
		folder=fmt.Sprintf("%s/%s",STORE_DIR,folder)
	}
	if fileInfo.Path!=""{
		if strings.HasPrefix(fileInfo.Path,STORE_DIR){
			folder=fileInfo.Path
		}else{
			folder=STORE_DIR+"/"+fileInfo.Path
		}
	}

	if !s.util.FileExists(folder){
		if err=os.MkdirAll(folder,0775);err!=nil{
			logrus.Errorf("mkdir error!folder=%s",folder)
		}
	}
	outPath:=fmt.Sprintf("%s/%s",folder,fileInfo.Name)
	if fileInfo.ReName!=""{
		outPath=fmt.Sprintf("%s/%s",folder,fileInfo.ReName)
	}
	if s.util.FileExists(outPath)&&Config().EnableDistinctFile{
		for i:=0;i<10000;i++{
			outPath=fmt.Sprintf("%s/%d_%s",folder,i,filepath.Base(header.Filename))
			fileInfo.Name=fmt.Sprintf("%d_%s",i,header.Filename)
			if !s.util.FileExists(outPath){
				break
			}
		}//for
	}

	logrus.Infof("upload:%s",outPath)
	if outFile,err=os.Create(outPath);err!=nil{
		return fileInfo,err
	}
	defer outFile.Close()
	if fi,err=outFile.Stat();err!=nil{
		logrus.Errorf("outFile stat error!err=%+v",err)
		return fileInfo, errors.New(fmt.Sprintf("(error) file fail!err=%+v",err))
	}else {
		fileInfo.Size=fi.Size()
	}
	if fi.Size()!=header.Size{
		return fileInfo,errors.New(fmt.Sprintf("(error) file incomplete!fi.size=%%d;header.size=%d;",fi.Size(),header.Size))
	}
	if Config().EnableDistinctFile{
		fileInfo.Md5=s.util.GetFileSum(outFile,Config().FileSumArithmetic)
	}else {
		fileInfo.Md5=s.util.MD5(s.GetFilePathByInfo(fileInfo,false))
	}
	fileInfo.Path=strings.Replace(folder,DOCKER_DIR,"",1)
	fileInfo.Peers=append(fileInfo.Peers,s.host)
	return fileInfo,nil
}

func (s *Server) Upload(w http.ResponseWriter,r *http.Request){
	var(
		err error
		fn string
		folder string
		fpTmp *os.File
		fpBody *os.File
	)
	if r.Method==http.MethodGet{
		//todo
		//s.upload(w,r);
		return
	}
	folder=STORE_DIR+"/_tmp/"+time.Now().Format("20060102")
	if s.util.FileExists(folder){
		if err=os.MkdirAll(folder,0777);err!=nil{
			logrus.Errorf("MkdirAll error!err=%+v",err)
		}
	}
	fn=folder+"/"+s.util.GetUUID()
	if fpTmp,err=os.OpenFile(fn,os.O_RDWR|os.O_CREATE,0777);err!=nil{
		logrus.Errorf("open file error!err=%+v",err)
		w.Write([]byte(err.Error()))
		return
	}
	defer fpTmp.Close()
	if _,err=io.Copy(fpTmp,r.Body);err!=nil{
		logrus.Errorf("copy error!err=%+v",err)
		w.Write([]byte(err.Error()))
		return
	}
	var fpBody *os.File
	if fpBody,err=os.OpenFile(fn,os.O_RDONLY,0);err!=nil{
		logrus.Errorf("open error!err=%+v",err)
		w.Write([]byte(err.Error()))
		return
	}
	r.Body=fpBody

	defer func(){
		if err=fpBody.Close();err!=nil{
			logrus.Errorf("fp Body close error!err=%+v",err)
		}
		if err=os.Remove(fn);err!=nil{
			logrus.Errorf("remove fn!err=%+v",err)
		}
	}()
	done:=make(chan bool,1)
	s.queueUpload<-WrapReqResp{
		w: &w,
		r:r,
		done: done,
	}
	<-done
}

func (s *Server) upload(w http.ResponseWriter,r *http.Request){
	var (
		err error
		ok bool
		md5sum,fileName,scene,output,code,msg string
		fileInfo FileInfo
		uploadFile multipart.File
		uploadHeader *multipart.FileHeader
		fileResult FileResult
		result JsonResult
		data []byte
		secret interface{}
	)
	output=r.FormValue("output")
	if Config().EnableCrossOrigin{
		s.CrossOrigin(w,r)
		if r.Method==http.MethodOptions{
			return
		}
	}
	result.Status="fail"
	if Config().AuthUrl!=""&&!s.CheckAuth(w,r){
		msg="auth fail"
		logrus.Warnf("msg=%s;r.Form=%+v;",msg,r.Form)
		s.NotPermit(w,r)
		result.Message=msg
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}

	if r.Method==http.MethodPost{
		md5sum=r.FormValue("md5")
		fileName=r.FormValue("filename")
		output=r.FormValue("output")
		if Config().ReadOnly{
			msg="(error) readonly"
			result.Message=msg
			logrus.Warnf("msg=%s",msg)
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}

		if Config().EnableCustomPath{
			fileInfo.Path=r.FormValue("path")
			fileInfo.Path=strings.Trim(fileInfo.Path,"/")
		}
		scene=r.FormValue("scene")
		code=r.FormValue("code")
		if scene==""{
			scene=r.FormValue("scenes")
		}
		if Config().EnableGoogleAuth&&scene!=""{
			if secret,ok=s.sceneMap.GetValue(scene);ok{
				//todo
				if !s.VerifyGoogleCode(secret.(string),code,int64(Config().DownloadTokenExpire/30)){
					s.NotPermit(w,r)
					result.Message="invalid request;google code error!"
					logrus.Errorf("google auth error!msg=%S;",result.Message)
					w.Write([]byte(s.util.JsonEncodePretty(result)))
					return
				}
			}
		}
		fileInfo.Md5=md5sum
		fileInfo.ReName=fileName
		fileInfo.OffSet=-1
		if uploadFile,uploadHeader,err=r.FormFile("file");err!=nil{
			logrus.Errorf("upload error!err=%+v",err)
			result.Message=err.Error()
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		fileInfo.Peers=[]string{}
		fileInfo.TimeStamp=time.Now().Unix()
		if scene==""{
			scene=Config().DefaultScene
		}
		if output==""{
			output="text"
		}
		if !s.util.Contains(output,[]string{"json","text","json2"}){
			msg="output just support json or text or json2"
			result.Message=msg
			logrus.Warnf("output unknown!output=%s",output)
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		fileInfo.Scene=scene
		if _,err=s.CheckScene(scene);err!=nil{
			result.Message=err.Error()
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			logrus.Errorf("CheckScene error!scene=%s;err=%+v;",scene,err)
			return
		}
		if _,err=s.SaveUploadFile(uploadFile,uploadHeader,&fileInfo,r);err!=nil{
			result.Message=err.Error()
			logrus.Errorf("err=%+v",err)
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		if Config().EnableDistinctFile{
			if v,_:=s.GetFileInfoFromLevelDB(fileInfo.Md5);v!=nil&&v.Md5!=""{
				fileResult=s.BuildFileResult(v,r)
				if s.GetFilePathByInfo(&fileInfo,false)!=s.GetFilePathByInfo(v,false){
					os.Remove(s.GetFilePathByInfo(&fileInfo,false))
				}
				if output=="json"||output=="json2"{
					if output=="json2"{
						result.Data=fileResult
						result.Status="ok"
						w.Write([]byte(s.util.JsonEncodePretty(result)))
						return
					}
					w.Write([]byte(s.util.JsonEncodePretty(fileResult)))
				}else {
					w.Write([]byte(fileResult.Url))
				}
				return
			}
		}
		if fileInfo.Md5==""{
			result.Message=" fileInfo.Md5 is null "
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			logrus.Errorf("fileInfo.md5 empty!")
			return
		}
		if !Config().EnableDistinctFile{
			fileInfo.Md5=s.util.MD5(s.GetFilePathByInfo(&fileInfo,false))
		}
		if Config().EnableMergeSmallFile&&fileInfo.Size<CONST_SMALL_FILE_SIZE{
			//todo
			if err=s.SaveSmallFile(&fileInfo);err!=nil{
				result.Message=err.Error()
				w.Write([]byte(s.util.JsonEncodePretty(result)))
				logrus.Errorf("SaveSmallFile error!err=%+v",err)
				return
			}
		}
		s.saveFileMd5Log(&fileInfo,CONST_FILE_Md5_FILE_NAME)
		go s.postFileToPeer(&fileInfo)
		if fileInfo.Size<=0{
			result.Message="file size is zero"
			logrus.Errorf("err_msg=%s",result.Message)
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		fileResult=s.BuildFileResult(&fileInfo,r)

		if output=="json2"{
			result.Data=fileResult
			result.Status="ok"
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}else if output=="json"{
			w.Write([]byte(s.util.JsonEncodePretty(fileResult)))
		}else {
			w.Write([]byte(fileResult.Url))
		}
		return
	}else {
		md5sum=r.FormValue("md5")
		output=r.FormValue("output")
		if md5sum==""{
			result.Message="(error) if you want to upload fast md5 is require,and if you want to upload file,you must use post method"
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			logrus.Errorf("md5sum null")
			return
		}
		if v,_:=s.GetFileInfoFromLevelDB(md5sum);v!=nil&&v.Md5!=""{
			fileResult=s.BuildFileResult(v,r)
			result.Data=fileResult
			result.Status="ok"
		}
		if output=="json"||output=="json2"{
			if data,err=json.Marshal(fileResult);err!=nil{
				result.Message=err.Error()
				w.Write([]byte(s.util.JsonEncodePretty(result)))
				logrus.Errorf("marshal error!err=%+v",err)
				return
			}
			if output=="json2"{
				w.Write([]byte(s.util.JsonEncodePretty(result)))
				return
			}
			w.Write(data)
		}else {
			w.Write([]byte(fileResult.Url))
		}
	}
}

func (s *Server) SaveSmallFile(fileInfo *FileInfo) error{
	var (
		err error
		fileName,fpath,largeDir,destPath,reName,fileExt string
		srcFile *os.File
		destFile *os.File
	)
	fileName=fileInfo.Name
	fileExt=path.Ext(fileName)
	if fileInfo.ReName!=""{
		fileName=fileInfo.ReName
	}
	fpath=DOCKER_DIR+fileInfo.Path+"/"+fileName
	largeDir=LARGE_DIR+"/"+Config().PeerId

	if !s.util.FileExists(largeDir){
		os.MkdirAll(largeDir,0775)
	}
	reName=fmt.Sprintf("%d",s.util.RandInt(100,300))
	destPath=largeDir+"/"+reName
	s.lockMap.LockKey(destPath)
	defer s.lockMap.UnLockKey(destPath)
	if s.util.FileExists(fpath){
		if srcFile,err=os.OpenFile(fpath,os.O_CREATE|os.O_RDONLY,06666);err!=nil{
			return err
		}
		defer func(){
			os.Remove(fpath)
			srcFile.Close()
		}()
		if destFile,err=os.OpenFile(destPath,os.O_CREATE|os.O_RDWR,06666);err!=nil{
			return err
		}
		defer destFile.Close()
		fileInfo.OffSet,err=destFile.Seek(0,2)
		if _,err=destFile.Write([]byte("1"));err!=nil{
			return err
		}
		fileInfo.OffSet--
		fileInfo.Size++
		fileInfo.ReName=fmt.Sprintf("%s,%d,%d,%s",reName,fileInfo.OffSet,fileInfo.Size,fileExt)
		if _,err=io.Copy(destFile,srcFile);err!=nil{
			return err
		}
		fileInfo.Path=strings.Replace(largeDir,DOCKER_DIR,"",1)
	}
	return nil
}

func (s *Server) SendToMail(to,subject,body,mailType string) error{
	host:=Config().Mail.Host
	user:=Config().Mail.User
	password:=Config().Mail.Password
	hp:=strings.Split(host,":")
	auth:=smtp.PlainAuth("",user,password,hp[0])

	contentType := "Content-Type: text/plain" + "; charset=UTF-8"
	if mailType=="html"{
		contentType = "Content-Type: text/" + mailType + "; charset=UTF-8"
	}
	msg := []byte("To: " + to + "\r\nFrom: " + user + ">\r\nSubject: " + "\r\n" + contentType + "\r\n\r\n" + body)
	recvs:=strings.Split(to,";")
	return smtp.SendMail(host,auth,user,recvs,msg)
}

func (s *Server) BenchMark(w http.ResponseWriter,r *http.Request){
	t:=time.Now()
	batch:=new(leveldb.Batch)
	n:=100000000
	for i:=0;i<n;i++{
		f:=FileInfo{
			Peers: []string{"http://192.168.0.1","http://192.168.2.5"},
			Path: "20190201/19/02",
		}
		md5str:=s.util.MD5(strconv.Itoa(i))
		f.Name=md5str
		f.Md5=md5str
		if data,err:=json.Marshal(&f);err==nil{
			batch.Put([]byte(md5str),data)
		}
		if i%10000==0{
			if batch.Len()>0{
				server.ldb.Write(batch,nil)
				batch.Reset()
			}
			logrus.Infof("i=%d;since_seconds=%d;",i,time.Since(t).Seconds())
		}
	}

	durationStr:=time.Since(t).String()
	s.util.WriteFile("time.txt",durationStr)
	logrus.Infof("durationStr=%s",durationStr)
}

func (s *Server) RepairStatWeb(w http.ResponseWriter, r *http.Request) {
	var (
		result JsonResult
		date   string
		inner  string
	)
	if !s.IsPeer(r) {
		result.Message = s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	date = r.FormValue("date")
	inner = r.FormValue("inner")
	if ok, err := regexp.MatchString("\\d{8}", date); err != nil || !ok {
		result.Message = fmt.Sprintf("invalid date!form_date=%s", date)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	if date == "" || len(date) != 8 {
		date = s.util.GetToDay()
	}
	if inner!="1"{
		for _,peer:=range Config().Peers{
			req:=httplib.Post(peer+s.getRequestURI("repair_stat"))
			req.Param("inner",inner)
			req.Param("date",date)
			if _,err:=req.String();err!=nil{
				logrus.Errorf("post error!err=%+v",err)
			}
		}
	}
	result.Data=s.RepairStatByDate(date)
	result.Status="ok"
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) Stat(w http.ResponseWriter,r *http.Request){
	var(
		result JsonResult
		inner,echart string
		category []string
		barCount,barSize []int64
		dataMap map[string]interface{}
	)

	if !s.IsPeer(r){
		result.Message=s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}

	r.ParseForm()
	inner=r.FormValue("inner")
	echart=r.FormValue("echart")
	//todo
	data:=s.GetStat()
	result.Status="ok"
	result.Data=data

	if echart=="1"{
		dataMap=make(map[string]interface{},3)

		for _,v:=range data{
			barCount=append(barCount,v.FileCount)
			barSize=append(barSize,v.TotalSize)
			category=append(category,v.Date)
		}
		dataMap["barCount"]=barCount
		dataMap["barSize"]=barSize
		dataMap["category"]=category
		result.Data=dataMap
	}

	if inner == "1" {
		w.Write([]byte(s.util.JsonEncodePretty(data)))
	} else{
		w.Write([]byte(s.util.JsonEncodePretty(result)))
	}
}

func (s *Server) GetStat() []StatDateFileInfo{
	var (
		min,max,i int64
		err error
		rows []StatDateFileInfo
		total StatDateFileInfo
	)
	min=20190101
	max=20190101
	for k:=range s.statMap.Get(){
		ks:=strings.Split(k,"_")
		if len(ks)==2{
			if i,err=strconv.ParseInt(ks[0],10,64);err!=nil{
				continue
			}
			if i>max{
				i=max
			}
			if i<min{
				i=min
			}
		}
	}//for
	for i:=min;i<=max;i++{
		str:=fmt.Sprintf("%d",i)

		if v,ok:=s.statMap.GetValue(str+"_"+CONST_STAT_FILE_COUNT_KEY);ok{
			info:=StatDateFileInfo{
				Date:str,
			}
			switch v.(type) {
			case int64:
				info.TotalSize=v.(int64)
				total.TotalSize+=v.(int64)
			}

			if v,ok:=s.statMap.GetValue(str+"_"+CONST_STAT_FILE_COUNT_KEY);ok{
				switch v.(type){
				case int64:
					info.FileCount=v.(int64)
					total.FileCount+=v.(int64)
				}
			}
			rows=append(rows,info)
		}
	}
	total.Date="all"
	rows=append(rows,total)
	return rows
}

func (s *Server) RegisterExit(){
	c:=make(chan os.Signal)
	signal.Notify(c,syscall.SIGHUP,syscall.SIGINT,syscall.SIGTERM,syscall.SIGQUIT)

	go func(){
		for sig:=range c{
			switch sig {
			case syscall.SIGHUP,syscall.SIGINFO,syscall.SIGTERM,syscall.SIGQUIT:
				s.ldb.Close()
				logrus.Infof("sig exit!sig=%+v",sig)
				os.Exit(1)
			}
		}
	}()
}

func (s *Server) AppendToQueue(fileInfo *FileInfo){
	for (len(s.queueToPeers)+CONST_QUEUE_SIZE/10)>CONST_QUEUE_SIZE{
		time.Sleep(time.Millisecond*50)
	}
	s.queueToPeers <- *fileInfo
}

func (s *Server) AppendToDownloadQueue(fileInfo *FileInfo){
	for (len(s.queueFromPeers)+CONST_QUEUE_SIZE/10)>CONST_QUEUE_SIZE{
		time.Sleep(time.Millisecond*50)
	}
	s.queueFromPeers<-*fileInfo
}

func (s *Server) ConsumerDownload(){
	for i:=0;i<Config().SyncWorker;i++{
		go func(){
			for{
				fileInfo:=<-s.queueFromPeers
				if len(fileInfo.Peers)<=0{
					logrus.Warnf("Peer is null!fileInfo=%+v",fileInfo)
					continue
				}
				for _,peer:=range fileInfo.Peers{
					if strings.Contains(peer,"127.0.0.1"){
						logrus.Warnf("sync error with 127.0.0.1!fileInfo=%+v",fileInfo)
						continue
					}
					if peer!=s.host{
						s.DownloadFromPeer(peer,&fileInfo)
						break
					}
				}

			}
		}()
	}
}


func (s *Server) RemoveDownloading(){
	go func(){
		for{
			it:=s.ldb.NewIterator(util.BytesPrefix([]byte("downloading_")),nil)
			for it.Next();{
				key:=it.Key()
				keys:=strings.Split(string(key),"_")
				if len(keys)==3{
					if t,err:=strconv.ParseInt(keys[1],10,64);err==nil&&(time.Now().Unix()-t>600){
						os.Remove(DOCKER_DIR+keys[2])
					}
				}
			}
			it.Release()
			time.Sleep(time.Minute*3)
		}
	}()
}

func (s *Server) ConsumerLog(){
	go func(){
		for{
			fileLog:=<-s.queueFileLog
			s.saveFileMd5Log(fileLog.FileInfo,fileLog.FileName)
		}
	}()
}

func (s *Server) LoadSearchDict(){
	go func(){
		logrus.Infof("LoadSearchDict ... ")
		var(
			f *os.File=nil
			err error
		)
		if f,err=os.OpenFile(CONST_SERACH_FILE_NAME,os.O_RDONLY,0);err!=nil{
			logrus.Errorf("open file error!CONST_SERACH_FILE_NAME=%s;err=%+v;",CONST_SERACH_FILE_NAME,err)
			return
		}
		defer f.Close()
		r:=bufio.NewReader(f)
		for{
			line,isPrefix,err:=r.ReadLine()
			for isPrefix&&err==nil{
				kvs:=strings.Split(string(line),"\t")
				if len(kvs)==2{
					s.searchMap.Put(kvs[0],kvs[1])
				}
			}

		}
		logrus.Infof("finish load search dict!")
	}()
}

func (s *Server) SaveSearchDict(){
	var(
		err error
		fp *os.File
		searchDict map[string]interface{}
		k string
		v interface{}
	)
	s.lockMap.LockKey(CONST_SEARCH_FILE_NAME)
	defer s.lockMap.UnLockKey(CONST_SEARCH_FILE_NAME)
	searchDict=s.searchMap.Get()
	if fp,err=os.OpenFile(CONST_SEARCH_FILE_NAME, os.O_RDWR,0755);err!=nil{
		logrus.Errorf("open file error!err=%+v",err)
		return
	}

	defer fp.Close()
	for k,v=range searchDict{
		fp.WriteString(fmt.Sprintf("%s\t%s",k,v.(string)))
	}
}

func (s *Server) ConsumerPostToPeer(){
	for i:=0;i<Config().SyncWorker;i++{
		go func(){
			for{
			  fileInfo :=<-s.queueToPeers
			  s.postFileToPeer(&fileInfo)
			}
		}()
	}
}

func (s *Server) ConsumerUpload(){
	for i:=0;i<Config().UploadWorker;i++{
		go func(){
			for{
				wr:=<-s.queueUpload
				s.upload(*wr.w,wr.r)
				s.rtMap.AddCountInt64(CONST_UPLOAD_COUNTER_KEY,wr.r.ContentLength)

				if v,ok:=s.rtMap.GetValue(CONST_UPLOAD_COUNTER_KEY);ok{
					if v.(int64)>(1<<30){
						var _v int64=0
						s.rtMap.Put(CONST_UPLOAD_COUNTER_KEY,_v)
						debug.FreeOSMemory()
					}
				}
				wr.done<-true
			}
		}()
	}
}


func (s *Server) updateInAutoRepairFunc(peer string,dateStat StatDateFileInfo){
	//远程拉来数据;
	req:=httplib.Get(fmt.Sprintf("%s%s?date=%s&force=%s",peer,s.getRequestURI("sync"),dateStat.Date,"1"))
	req.SetTimeout(time.Second*5,time.Second*5)

	if _,err:=req.String();err!=nil{
		logrus.Errorf("request error!err=%+v",err)
	}
	logrus.Infof(fmt.Sprintf("sync file from %s date %s",peer,dateStat.Date))
}

func (s *Server) autoRepairFunc(forceRepair bool){
	var(
		dateStats []StatDateFileInfo
		err error
		countKey,md5s string
		localSet,remoteSet,allSet,tmpSet mapset.Set
		fileInfo *FileInfo
	)
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("autoRepairFunc")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()

	for _,peer:=range Config().Peers{
		req:=httplib.Post(fmt.Sprintf("%s%s",peer,s.getRequestURI("stat")))
		req.Param("inner","1")
		req.SetTimeout(time.Second*5,time.Second*15)
		if err=req.ToJSON(&dateStats);err!=nil{
			logrus.Errorf("req error!err=%+v",err)
			continue
		}

		for _,dateStat:=range dateStats{
			if dateStat.Date=="all"{
				continue
			}

			countKey=dateStat.Date+"_"+CONST_STAT_FILE_COUNT_KEY
			if v,ok:=s.statMap.GetValue(countKey);ok{
				switch v.(type) {
				case int64:
					if v.(int64)==dateStat.FileCount||forceRepair{
						//不相等,找差异;
						req:=httplib.Post(fmt.Sprintf("%s%s",peer,s.getRequestURI("get_md5s_by_date")))
						req.SetTimeout(time.Second*15,time.Second*60)
						req.Param("date",dateStat.Date)
						if md5s,err=req.String();err!=nil{
							continue
						}
						if localSet,err=s.GetMd5sByDate(dateStat.Date,CONST_FILE_Md5_FILE_NAME);err!=nil{
							logrus.Errorf("GetMd5sMapByDate error!date=%s;err=%+v;",dateStat.Date,err)
							continue
						}
						remoteSet=s.util.StrToMapSet(md5s,",")
						allSet=localSet.Union(remoteSet)
						md5s=s.util.MapSetToStr(allSet.Difference(localSet),",")

						req=httplib.Post(fmt.Sprintf("%s%s",peer,s.getRequestURI("receive_md5s")))
						req.SetTimeout(time.Second*15,time.Second*60)
						req.Param("md5s",md5s)
						req.String()
						tmpSet=allSet.Difference(remoteSet)
						for v:=range tmpSet.Iter(){
							if v!=nil{
							   if fileInfo,err=s.GetFileInfoFromLevelDB(v.(string));err!=nil{
							   	logrus.Errorf("GetFileInfoFromLevelDB error!v=%+v;err=%+v",v,err)
							   	continue
							   }
							   s.AppendToQueue(fileInfo)
							}
						}//for
					}
				}
			}else {
				s.updateInAutoRepairFunc(peer,dateStat)
			}
		}//for
	}//for
}


func (s *Server) AutoRepair(forceRepair bool){
	if s.lockMap.IsLock("AutoRepair"){
		logrus.Warnf("AutoRepair has been locked!")
		return
	}
	s.lockMap.LockKey("AutoRepair")
	defer s.lockMap.UnLockKey("AutoRepair")
	//AutoRepairFunc
	s.autoRepairFunc(forceRepair)
}

func (s *Server) CleanLogLevelDBByDate(date string,fileName string){
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("autoRepairFunc")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var (
		err error
		keyPrefix string
		keys mapset.Set
	)
	keys=mapset.NewSet()
	keyPrefix=fmt.Sprintf("%s_%s_",date,fileName)
	it:=server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)),nil)
	defer it.Release()
	for it.Next(){
		keys.Add(string(it.Value()))
	}
	for key:=range keys.Iter(){
		if err=s.RemoveKeyFromLevelDB(key.(string),s.logDB);err!=nil{
			logrus.Errorf("RemoveKeyFromLevelDB error!err=%+v",err)
		}
	}
}

func (s *Server) CleanAndBackup(){
	go func(){
		for{
			time.Sleep(time.Minute*2)
			var (
				fileNames []string
				yesterday string
			)
			if s.curDate!=s.util.GetToDay(){
				fileNames=[]string{CONST_Md5_QUEUE_FILE_NAME,CONST_Md5_ERROR_FILE_NAME,CONST_REMOVE_Md5_FILE_NAME}
				yesterday=s.util.GetDayFromTimeStamp(time.Now().AddDate(0,0,-1).Unix())
				for _,fileName:=range fileNames{
					s.CleanLogLevelDBByDate(yesterday,FileName)
				}
				s.BackUpMetaDataByDate(yesterday)
				s.curDate=s.util.GetToDay()
			}
		}
	}()
}

func (s *Server) LoadFileInfoByDate(date string,fileName string) (mapset.Set,error){
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("autoRepairFunc")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var(
		err error
		keyPrefix string
		fileInfos mapset.Set
	)
	fileInfos=mapset.NewSet()
	keyPrefix=fmt.Sprintf("%s_%s_",date,fileName)
	it:=server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)),nil)
	defer it.Release()
	for it.Next(){
		var fileInfo FileInfo
		if err=json.Unmarshal(it.Value(),&fileInfo);err!=nil{
			logrus.Warnf("umarshal error!v=%+v",it.Value())
			continue
		}
		fileInfos.Add(&fileInfo)
	}
	return fileInfos,nil
}


func (s *Server) LoadQueueSendToPeer(){
	if queue,err:=s.LoadFileInfoByDate(s.util.GetToDay(),CONST_Md5_QUEUE_FILE_NAME);err!=nil{
		logrus.Errorf("LoadFileInfoByDate error!date=%+v",s.util.GetToDay())
	}else {
		for fileInfo:=range queue.Iter(){
			s.AppendToDownloadQueue(fileInfo.(*FileInfo))
		}
	}
}

























































