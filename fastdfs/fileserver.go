package fastdfs
/**
参考:
https://gitee.com/linux2014/go-fastdfs_2
 */

import (
	"flag"
	"fmt"
	"github.com/radovskyb/watcher"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
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
	curData string
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
	CONST_SERACH_FILE_NAME=DATA_DIR+"/search.txt"
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









