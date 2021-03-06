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
	mapset "github.com/deckarep/golang-set"
	"github.com/nfnt/resize"
	"github.com/radovskyb/watcher"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/sirupsen/logrus"
	"github.com/sjqzhang/googleAuthenticator"
	"github.com/sjqzhang/tusd/filestore"

	//"github.com/tus/tusd"
	"github.com/sjqzhang/tusd"
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
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/astaxie/beego/httplib"
	_ "github.com/eventials/go-tus"
	//jsoniter "github.com/json-iterator/go"
	"github.com/sjqzhang/goutil"
	log "github.com/sjqzhang/seelog"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func NewServer() *Server {
	if server != nil {
		return server
	}

	server := &Server{
		util:           &goutil.Common{},
		statMap:        goutil.NewCommonMap(0),
		lockMap:        goutil.NewCommonMap(0),
		rtMap:          goutil.NewCommonMap(0),
		sceneMap:       goutil.NewCommonMap(0),
		searchMap:      goutil.NewCommonMap(0),
		queueToPeers:   make(chan FileInfo, CONST_QUEUE_SIZE),
		queueFromPeers: make(chan FileInfo, CONST_QUEUE_SIZE),
		queueFileLog:   make(chan *FileLog, CONST_QUEUE_SIZE),
		queueUpload:    make(chan WrapReqResp, 100),
		sumMap:         goutil.NewCommonMap(365 * 3),
	}
	defaultTransport := &http.Transport{
		DisableKeepAlives:   true,
		Dial:                httplib.TimeoutDialer(time.Second*15, time.Second*300),
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
	}
	settings := httplib.BeegoHTTPSettings{
		UserAgent:        "Go-FastDFS",
		ConnectTimeout:   15 * time.Second,
		ReadWriteTimeout: 15 * time.Second,
		Gzip:             true,
		DumpBody:         true,
		Transport:        defaultTransport,
	}

	httplib.SetDefaultSetting(settings)

	server.statMap.Put(CONST_STAT_FILE_COUNT_KEY, int64(0))
	server.statMap.Put(CONST_STAT_FILE_TOTAL_SIZE_KEY, int64(0))
	server.statMap.Put(server.util.GetToDay()+"_"+CONST_STAT_FILE_COUNT_KEY, int64(0))
	server.statMap.Put(server.util.GetToDay()+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY, int64(0))
	server.curDate = server.util.GetToDay()

	opts := &opt.Options{
		CompactionTableSize: 1024 * 1024 * 20,
		WriteBuffer:         1024 * 1024 * 20,
	}
	var err error
	server.logDB, err = leveldb.OpenFile(CONST_LOG_LEVELDB_FILE_NAME, opts)
	if err != nil {
		logrus.Errorf("open leveldb file %s fail,maybe has been opening!err:=%+v\n", CONST_LOG_LEVELDB_FILE_NAME, err)
		panic(err)
	}

	return server
}

//need test;
func Config() *GlobalConfig {
	return (*GlobalConfig)(atomic.LoadPointer(&ptr))
}

/**
用默认的配置或指定的配置文件;
*/
func ParseConfig(filePath string) {
	var data []byte
	if filePath == "" {
		data = []byte(strings.TrimSpace(cfgJson))
	} else {
		file, err := os.Open(filePath)

		if err != nil {
			panic(fmt.Sprintln("open file path:", filePath, "error:", err))
		}
		defer file.Close()
		FileName = filePath
		data, err = ioutil.ReadAll(file)
		if err != nil {
			panic(fmt.Sprintln("file path:", filePath, " read all error: ", err))
		}

	}
	var c GlobalConfig
	if err := json.Unmarshal(data, &c); err != nil {
		panic(fmt.Sprintln("file path:", filePath, " json unmarshal error:", err))
	}
	logrus.Infof("c=%+v\n", c)
	atomic.StorePointer(&ptr, unsafe.Pointer(&c))
	logrus.Infof("config parse success!")
}

/**
need test
按日期备份元数据;
*/
func (s *Server) BackUpMetaDataByDate(date string) {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			logrus.Errorf("BackUpMetaDataByDate!re:%+v;buffer:%s\n", re, string(buffer))
		}
	}()

	var (
		err          error
		keyPrefix    string
		msg          string
		name         string
		fileInfo     FileInfo
		logFileName  string
		fileLog      *os.File
		fileMeta     *os.File
		metaFileName string
		fi           os.FileInfo
	)
	logFileName = DATA_DIR + "/" + date + "/" + CONST_FILE_Md5_FILE_NAME
	s.lockMap.LockKey(logFileName)
	defer s.lockMap.UnLockKey(logFileName)
	metaFileName = DATA_DIR + "/" + date + "/" + "meta.data"
	os.MkdirAll(DATA_DIR+"/"+date, 0775)

	if s.util.IsExist(logFileName) {
		os.Remove(logFileName)
	}

	if s.util.IsExist(metaFileName) {
		os.Remove(metaFileName)
	}

	if fileLog, err = os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0664); err != nil {
		logrus.Errorf("openFile(%s) error(%+v)\n", logFileName, err)
		return
	}
	defer fileLog.Close()

	if fileMeta, err = os.OpenFile(metaFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0664); err != nil {
		logrus.Errorf("openFile(%s) error(%+v)\n", logFileName, err)
		return
	}
	defer fileMeta.Close()

	keyPrefix = fmt.Sprintf("%s_%s", date, CONST_FILE_Md5_FILE_NAME)
	it := server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)), nil)
	defer it.Release()

	for it.Next() {
		if err = json.Unmarshal(it.Value(), &fileInfo); err != nil {
			logrus.Errorf("unmarshal error!it=%+v\n", it)
			continue
		}
		if fileInfo.ReName != "" {
			name = fileInfo.ReName
		} else {
			name = fileInfo.Name
		}
		msg = fmt.Sprintf("%s\t%s\n", fileInfo.Md5, string(it.Value()))
		if _, err = fileMeta.WriteString(msg); err != nil {
			logrus.Errorf("fileMeta(%+v) write error!msg=%s;err=%+v\n", fileMeta, msg, err)
		}

		msg = fmt.Sprintf("%s\t%s\n", s.util.MD5(fileInfo.Path+"/"+name), string(it.Value()))
		if _, err = fileMeta.WriteString(msg); err != nil {
			logrus.Errorf("fileMeta(%+v) write error!msg=%s;err=%+v\n", fileMeta, msg, err)
		}

		msg = fmt.Sprintf("%s|%d|%d|%s\n", fileInfo.Md5, fileInfo.Size, fileInfo.TimeStamp, fileInfo.Path+"/"+name)
		if _, err = fileMeta.WriteString(msg); err != nil {
			logrus.Errorf("fileMeta(%+v) write error!msg=%s;err=%+v\n", fileMeta, msg, err)
		}
	}

	if fi, err = fileLog.Stat(); err != nil {
		logrus.Errorf("fileLog stat error!err=%+v\n", err)
	} else if fi.Size() == 0 {
		fileLog.Close()
		os.Remove(logFileName)
	}

	if fi, err = fileMeta.Stat(); err != nil {
		logrus.Errorf("fileMeta stat error!err=%+v\n", err)
	} else if fi.Size() == 0 {
		fileMeta.Close()
		os.Remove(metaFileName)
	}
}

var globalServer *Server
var pathPrefix string

func handleFunc(filePath string, f os.FileInfo, err error) error {
	var (
		files    []os.FileInfo
		fi       os.FileInfo
		fileInfo FileInfo
		sum      string
		pathMd5  string
	)
	if f.IsDir() {
		if files, err = ioutil.ReadDir(filePath); err != nil {
			logrus.Errorf("read_dir error!filePath=%s;err=%+v\n", filePath, err)
			return err
		}
		for _, fi = range files {
			if fi.Size() == 0 || fi.IsDir() {
				continue
			}
			filePath = strings.Replace(filePath, "\\", "/", -1)
			if DOCKER_DIR != "" {
				filePath = strings.Replace(filePath, DOCKER_DIR, "", 1)
			}
			if pathPrefix != "" {
				filePath = strings.Replace(filePath, pathPrefix, STORE_DIR_NAME, 1)
			}
			if strings.HasPrefix(filePath, STORE_DIR_NAME+"/"+LARGE_DIR_NAME) {
				logrus.Infof(fmt.Sprintf("ignore small file file %s!", filePath+"/"+fi.Name()))
				continue
			}
			pathMd5 = globalServer.util.MD5(filePath + "/" + fi.Name())
			sum = pathMd5

			if err != nil {
				logrus.Errorf("err=%+v\n", err)
				continue
			}
			fileInfo = FileInfo{
				Size:      fi.Size(),
				Name:      fi.Name(),
				Path:      filePath,
				Md5:       sum,
				TimeStamp: fi.ModTime().Unix(),
				Peers:     []string{globalServer.host},
				OffSet:    -2,
			}
			logrus.Infof("fileInfo=%+v\n", fileInfo)
			//todo
			//s.AppendToQueue(&fileInfo)
			//s.postFileToPeer(&fileInfo)
			//s.SaveFileInfoToLevelDB(fileInfo.Md5,&fileInfo,s.ldb)
			//s.SaveFileMd5Log(&fileInfo,CONST_FILE_Md5_FILE_NAME)
		}
	}
	return nil
}

func (s *Server) RepairFileInfoFromFile() {
	var (
		//pathPrefix string
		err error
		fi  os.FileInfo
	)
	globalServer = s

	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			logrus.Errorf("RepairFileInfoFromFile error!re=%+v;buffer=%s\n", re, string(buffer))
		}
	}()

	if s.lockMap.IsLock("RepairFileInfoFromFile") {
		logrus.Warnf("Lock RepairFileInfoFromFile")
		return
	}

	s.lockMap.LockKey("RepairFileInfoFromFile")
	defer s.lockMap.UnLockKey("RepairFileInfoFromFile")
	//handlefunc
	pathname := STORE_DIR
	if pathPrefix, err = os.Readlink(pathname); err == nil {
		pathname = pathPrefix

		if strings.HasSuffix(pathPrefix, "/") {
			pathPrefix = pathPrefix[0 : len(pathPrefix)-1]
		}
	}
	if fi, err = os.Stat(pathname); err != nil {
		logrus.Errorf("stat error!pathname=%s;err=%+v;\n", pathname, err)
	}
	if fi.IsDir() {
		filepath.Walk(pathname, handleFunc)
	}
	logrus.Infof("RepairFileInfoFromFile finish!")
}

func (s *Server) WatchFilesChange() {
	var (
		w      *watcher.Watcher
		curDir string
		err    error
		qchan  chan *FileInfo
		isLink bool
	)
	qchan = make(chan *FileInfo, Config().WatchChanSize)
	w = watcher.New()
	w.FilterOps(watcher.Create)

	if curDir, err = filepath.Abs(filepath.Dir(STORE_DIR_NAME)); err != nil {
		logrus.Errorf("file error!err=%+v\n", err)
	}
	go func() {
		//事件监控协程;
		for {
			select {
			case event := <-w.Event:
				logrus.Infof("event=%+v\n", event)
				if event.IsDir() {
					continue
				}
				fpath := strings.Replace(event.Path, curDir+string(os.PathSeparator), "", 1)

				if isLink {
					fpath = strings.Replace(event.Path, curDir, STORE_DIR_NAME, 1)
				}
				fpath = strings.Replace(fpath, string(os.PathSeparator), "/", -1)
				sum := s.util.MD5(fpath)
				fileInfo := FileInfo{
					Size:      event.Size(),
					Name:      event.Name(),
					Path:      strings.TrimSuffix(fpath, "/"+event.Name()),
					Md5:       sum,
					TimeStamp: event.ModTime().Unix(),
					Peers:     []string{s.host},
					OffSet:    -2,
					op:        event.Op.String(),
				}
				logrus.Infof(fmt.Sprintf("WatchFilesChange op:%s path:%s", event.Op.String(), fpath))
				//一旦有事件发生,则将fileInfo加入qchan;
				qchan <- &fileInfo
				break
			case err = <-w.Error:
				logrus.Errorf("err=%+v\n", err)
				break
			case v := <-w.Closed:
				logrus.Infof("close!v=%+v\n", v)
				return
			} //select;
		} //for
	}() //go func();

	go func() {
		//处理qchan内的事件;
		for {
			c := <-qchan
			if time.Now().Unix()-c.TimeStamp < Config().SyncDelay {
				qchan <- c
				time.Sleep(time.Second)
				continue
			} else {
				if c.op == watcher.Create.String() {
					logrus.Infof(fmt.Sprintf("Syncfile Add to queue path:%s\n", c.Path+"/"+c.Name))
					//todo
					//s.AppendToQueue(c)
					//s.SaveFileInfoToLevelDB(c.Md5,c,s.ldb)
				}
			}
		} //for
	}() //go func()

	if dir, err := os.Readlink(STORE_DIR_NAME); err == nil {
		if strings.HasSuffix(dir, string(os.PathSeparator)) {
			dir = strings.TrimSuffix(dir, string(os.PathSeparator))
		}
		curDir = dir
		isLink = true
		if err := w.AddRecursive(dir); err != nil {
			logrus.Errorf("AddRecursive err=%+v\n", v)
		}
		w.Ignore(dir + "/_tmp/")
		w.Ignore(dir + "/" + LARGE_DIR_NAME + "/")
	}
	if err := w.AddRecursive("./" + STORE_DIR_NAME); err != nil {
		logrus.Errorf("AddRecursive err=%+v\n", v)
	}
	w.Ignore("./" + STORE_DIR_NAME + "/_tmp/")
	w.Ignore("./" + STORE_DIR_NAME + "/" + LARGE_DIR_NAME + "/")
	if err := w.Start(time.Millisecond * 100); err != nil {
		logrus.Errorf("watcher start error!err=%+v\n", err)
	}
}

func (s *Server) RepairStatByDate(date string) StatDateFileInfo {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			logrus.Errorf("RepairStatByDate;re=%+v;buffer=%+v\n", re, string(buffer))
		}
	}()
	var (
		err       error
		keyPrefix string
		fileInfo  FileInfo
		fileCount int64
		fileSize  int64
		stat      StatDateFileInfo
	)
	keyPrefix = fmt.Sprintf("%s_%s_", date, CONST_FILE_Md5_FILE_NAME)
	it := s.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)), nil)
	defer it.Release()
	for it.Next() {
		if err = json.Unmarshal(it.Value(), &fileInfo); err != nil {
			continue
		}
		fileCount++
		fileSize += fileInfo.Size
	}
	s.statMap.Put(date+"_"+CONST_STAT_FILE_COUNT_KEY, fileCount)
	s.statMap.Put(date+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY, fileSize)
	//todo
	//s.SaveStat()
	stat.Date = date
	stat.FileCount = fileCount
	stat.TotalSize = fileSize
	return stat
}

func (s *Server) GetFilePathByInfo(fileInfo *FileInfo, withDocker bool) string {
	fn := fileInfo.Name
	if fileInfo.ReName != "" {
		fn = fileInfo.ReName
	}
	if withDocker {
		return DOCKER_DIR + fileInfo.Path + "/" + fn
	}
	return fileInfo.Path + "/" + fn
}

func (s *Server) CheckFileExistByInfo(md5s string, fileInfo *FileInfo) bool {
	var (
		err      error
		fullPath string
		fi       os.FileInfo
		info     *FileInfo
	)
	if fileInfo == nil {
		return false
	}

	if fileInfo.OffSet >= 0 {
		//small file;
		/**
		if info,err=s.GetFileInfoFromLevelDB(fileInfo.Md5);err==nil&&info.Md5==fileInfo.Md5{
		   return true
		}else{
		   return false
		}
		*/
	}

	fullPath = s.GetFilePathByInfo(fileInfo, true)
	if fi, err = os.Stat(fullPath); err != nil {
		return false
	}
	if fi.Size() == fileInfo.Size {
		return true
	} else {
		return false
	}
}

func (s *Server) ParseSmallFile(fileName string) (string, int64, int, error) {
	var (
		err    error
		offset int64
		length int
	)
	err = errors.New("invalid small file")
	if len(fileName) < 3 {
		return fileName, -1, -1, err
	}
	if strings.Contains(fileName, "/") {
		fileName = fileName[strings.LastIndex(fileName, "/"):]
	}
	pos := strings.Split(fileName, ",")
	if len(pos) < 3 {
		return fileName, -1, -1, err
	}
	if offset, err = strconv.ParseInt(pos[1], 10, 64); err != nil {
		return fileName, -1, -1, err
	}
	if length, err = strconv.Atoi(pos[2]); err != nil {
		return fileName, offset, -1, err
	}
	if length > CONST_SMALL_FILE_SIZE || offset < 0 {
		err = errors.New("invalid filesize pf offset")
		return fileName, -1, -1, err
	}
	return pos[0], -1, -1, err
}

func (s *Server) DownloadFromPeer(peer string, fileInfo *FileInfo) {
	var (
		err         error
		fileName    string
		fpath       string
		fpathTmp    string
		fi          os.FileInfo
		sum         string
		data        []byte
		downloadUrl string
	)
	if Config().ReadOnly {
		logrus.Warnf("Readonly; fileInfo=%+v\n", fileInfo)
		return
	}
	if Config().RetryCount > 0 && fileInfo.retry >= Config().RetryCount {
		logrus.Errorf("DownloadFromPeer error!fileInfo=%+v\n", fileInfo)
		return
	} else {
		fileInfo.retry++
	}
	fileName = fileInfo.Name
	if fileInfo.ReName != "" {
		fileName = fileInfo.ReName
	}
	if fileInfo.OffSet != -2 && Config().EnableDistinctFile && s.CheckFileExistByInfo(fileInfo.Md5, fileInfo) {
		//ignore migrate file
		logrus.Infof("DownloadFromPeer file exist;path=%+v\n", fileInfo.Path+"/"+fileInfo.Name)
		return
	}
	if (!Config().EnableDistinctFile || fileInfo.OffSet == -2) || s.util.FileExists(s.GetFilePathByInfo(fileInfo, true)) {
		if fi, err = os.Stat(s.GetFilePathByInfo(fileInfo, true)); err != nil {
			logrus.Infof("ignore file sync path:%s\n", s.GetFilePathByInfo(fileInfo, false))
			fileInfo.TimeStamp = fi.ModTime().Unix()
			//to do
			//s.PostFileToPeer(fileInfo);
			return
		}
		os.Remove(s.GetFilePathByInfo(fileInfo, true))
	}

	if _, err = os.Stat(fileInfo.Path); err != nil {
		os.MkdirAll(DOCKER_DIR+fileInfo.Path, 0775)
	}

	p := strings.Replace(fileInfo.Path, STORE_DIR_NAME+"/", "", 1)

	if Config().SupportGroupManage {
		downloadUrl = peer + "/" + Config().Group + "/" + p + "/" + fileName
	} else {
		downloadUrl = peer + "/" + p + "/" + fileName
	}
	logrus.Infof("DownloadFromPeer url=%s\n", downloadUrl)
	fpath = DOCKER_DIR + fileInfo.Path + "/" + fileName
	fpathTmp = DOCKER_DIR + fileInfo.Path + "/tmp_" + fileName
	timeout := fileInfo.Size>>20 + 30
	if Config().SyncTimeout > 0 {
		timeout = Config().SyncTimeout
	}
	s.lockMap.LockKey(fpath)
	defer s.lockMap.UnLockKey(fpath)
	downloadKey := fmt.Sprintf("downloading_%d_%s", time.Now().Unix(), fpath)
	s.ldb.Put([]byte(downloadKey), []byte(""), nil)
	defer func() {
		s.ldb.Delete([]byte(downloadKey), nil)
	}()

	if fileInfo.OffSet == -2 {
		//migrate
		if fi, err = os.Stat(fpath); err == nil && fi.Size() == fileInfo.Size {
			//todo
			//s.SaveFileInfoTo:LevelDB(fileInfo.Md5,fileInfo,fpath)
			return
		}
		req := httplib.Get(downloadUrl)
		req.SetTimeout(time.Second*30, time.Second*time.Duration(timeout))
		if err = req.ToFile(fpathTmp); err != nil {
			//todo
			//s.AppendToDowndQueue(fileInfo);//retry
			os.Remove(fpathTmp)
			logrus.Errorf("req.ToFile error!fpathTmp=%s;err=%+v;", fpathTmp, err)
			return
		}

		if fi, err = os.Stat(fpathTmp); err != nil {
			os.Remove(fpathTmp)
			return
		} else if fi.Size() != fileInfo.Size {
			logrus.Errorf("file size check error!fi.Name=%s;fileInfo.Name=%s;", fi.Name(), fileInfo.Name)
			os.Remove(fpathTmp)
		}
		if os.Rename(fpathTmp, fpath) == nil {
			//todo
			//s.SaveFileInfoToLevelDB(fileInfo.Md5,fileInfo,s.ldb)
		}
		return
	}
	req := httplib.Get(downloadUrl)
	req.SetTimeout(time.Second*30, time.Second*time.Duration(timeout))

	if fileInfo.OffSet >= 0 {
		//small file download
		if data, err = req.Bytes(); err != nil {
			//todo
			//s.AppendToDownloadQueue(fileInfo)
			logrus.Errorf("download error!err=%+v\n", err)
			return
		}
		data2 := make([]byte, len(data)+1)
		data2[0] = '1'
		for i, v := range data {
			data2[i+1] = v
		}
		data = data2

		if int64(len(data)) != fileInfo.Size {
			logrus.Errorf("file size error!")
			return
		}

		fpath = strings.Split(fpath, ",")[0]
		if err = s.util.WriteFileByOffSet(fpath, fileInfo.OffSet, data); err != nil {
			logrus.Errorf("WriteFileByOffSet error!fpath=%s;err=%+v\n", fpath, err)
			return
		}
		//todo
		//s.SaveFileMd5Log(fileInfo,CONST_FILE_Md5_FILE_NAME)
	}
	if err = req.ToFile(fpathTmp); err != nil {
		//todo
		//s.AppendToDownloadQueue(fileInfo);
		os.Remove(fpathTmp)
		logrus.Errorf("req.ToFile error!fpathTmp=%s;err=%+v;", fpathTmp, err)
		return
	}
	if fi.Size() != fileInfo.Size {
		logrus.Errorf("file sum check error!")
		os.Remove(fpathTmp)
		return
	}

	if os.Rename(fpathTmp, fpath) == nil {
		//todo
		//s.SaveFileMd5Log(fileInfo,CONST_FILE_Md5_FILE_NAME);
	}
}

func (s *Server) CrossOrigin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Depth, User-Agent, X-File-Size, X-Requested-With, X-Requested-By, If-Modified-Since, X-File-Name, X-File-Type, Cache-Control, Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Expose-Headers", "Authorization")
	//https://blog.csdn.net/yanzisu_congcong/article/details/80552155
}

func (s *Server) SetDownloadHeader(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment")
	if name, ok := r.URL.Query()["name"]; ok {
		if v, err := url.QueryUnescape(name[0]); err == nil {
			name[0] = v
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s", name[0]))
	}
}

func (s *Server) CheckAuth(w http.ResponseWriter, r *http.Request) bool {
	var (
		err        error
		req        *httplib.BeegoHTTPRequest
		result     string
		jsonResult JsonResult
	)
	if err = r.ParseForm(); err != nil {
		logrus.Errorf("ParseForm error!errr=%+v\n", err)
		return false
	}
	req = httplib.Post(Config().AuthUrl)
	req.SetTimeout(time.Second*10, time.Second*10)
	req.Param("__path__", r.URL.Path)
	req.Param("__query__", r.URL.RawQuery)
	for k, v := range r.Form {
		logrus.Infof("form;k=%s;v=%s\n", k, v)
		req.Param(k, r.FormValue(k))
	}
	for k, v := range r.Header {
		logrus.Infof("header;k=%s;v=%s\n", k, v)
		req.Header(k, v[0])
	}
	result, err = req.String()
	result = strings.TrimSpace(result)
	if strings.HasPrefix(result, "{") && strings.HasSuffix(result, "}") {
		if err = json.Unmarshal([]byte(result), &jsonResult); err != nil {
			logrus.Errorf("unmarshal error!err=%+v\n", err)
			return false
		}
		if jsonResult.Data != "ok" {
			logrus.Errorf("result not ok!")
			return false
		}
	} else if result != "ok" {
		logrus.Warnf("result not ok!")
		return false
	}
	return true
}

func (s *Server) NotPermit(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(401)
}

func (s *Server) GetFilePathFromRequest(w http.ResponseWriter, r *http.Request) (string, string) {
	var (
		err       error
		fullPath  string
		smallPath string
		prefix    string
	)
	fullPath = r.RequestURI[1:]

	if strings.HasPrefix(r.RequestURI, "/"+Config().Group+"/") {
		fullPath = r.RequestURI[len(Config().Group)+2 : len(r.RequestURI)]
	}
	fullPath = strings.Split(fullPath, "?")[0] //just path
	fullPath = DOCKER_DIR + STORE_DIR_NAME + "/" + fullPath
	prefix = "/" + LARGE_DIR_NAME + "/"

	if Config().SupportGroupManage {
		prefix = "/" + Config().Group + "/" + LARGE_DIR_NAME + "/"
	}
	if strings.HasPrefix(r.RequestURI, prefix) {
		smallPath = fullPath
		fullPath = strings.Split(fullPath, ",")[0]
	}
	if fullPath, err = url.PathUnescape(fullPath); err != nil {
		logrus.Errorf("pathUnescape error!err=%+v\n", err)
	}
	return fullPath, smallPath
}

func (s *Server) CheckDownloadAuth(w http.ResponseWriter, r *http.Request) (bool, error) {
	var (
		err          error
		maxTimestamp int64
		minTimestamp int64
		ts           int64
		token        string
		timestamp    string
		fullPath     string
		smallPath    string
		pathMd5      string
		fileInfo     *FileInfo
		scene        string
		secret       interface{}
		code         string
		ok           bool
	)
	CheckToken := func(token string, md5sum string, timestamp string) bool {
		if s.util.MD5(md5sum+timestamp) != token {
			return false
		} else {
			return true
		}
	}

	//todo
	//&&!s.IsPeer(r)&&!s.CheckAuth(w,r)
	if Config().EnableDownloadAuth && Config().AuthUrl != "" {
		return false, errors.New("auth fail!")
	}
	//todo
	//&& s.IsPeer(r)
	if Config().DownloadUseToken {
		token = r.FormValue("token")
		timestamp = r.FormValue("timestamp")

		if token == "" || timestamp == "" {
			return false, errors.New("invalid request!need token and timestamp!")
		}

		maxTimestamp = time.Now().Add(time.Second * time.Duration(Config().DownloadTokenExpire)).Unix()
		minTimestamp = time.Now().Add(-time.Second * time.Duration(Config().DownloadTokenExpire)).Unix()
		if ts, err = strconv.ParseInt(timestamp, 10, 64); err != nil {
			return false, errors.New(fmt.Sprintf("invalid timestamp!timestamp=%+v\n", timestamp))
		}
		if ts < minTimestamp || ts > maxTimestamp {
			return false, errors.New(fmt.Sprintf("timestamp expire!ts=%d", ts))
		}
		fullPath, smallPath = s.GetFilePathFromRequest(w, r)
		if smallPath != "" {
			pathMd5 = s.util.MD5(smallPath)
		} else {
			pathMd5 = s.util.MD5(fullPath)
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
	if Config().EnableGoogleAuth {
		fullPath = r.RequestURI[len(Config().Group)+2:]
		fullPath = strings.Split(fullPath, "?")[0]
		scene = strings.Split(fullPath, "/")[0]
		code = r.FormValue("code")

		if secret, ok = s.sceneMap.GetValue(scene); ok {
			//todo
			/*			if !s.VerifyGoogleCode(secret.(string),code,int64(Config().DownloadTokenExpire/30)){
						return false,errors.New(fmt.Sprintf("invalid google code!scene=%+v;secret=%+v;code=%+v;",scene,secret,code))
					}*/
		}

	}
	return true, nil
}

func (s *Server) GetSmallFileByURI(w http.ResponseWriter, r *http.Request) ([]byte, bool, error) {
	var (
		err      error
		data     []byte
		offset   int64
		length   int
		fullPath string
		info     os.FileInfo
	)
	fullPath, _ = s.GetFilePathFromRequest(w, r)

	if _, offset, length, err = s.ParseSmallFile(r.RequestURI); err != nil {
		return nil, false, err
	}
	if info, err = os.Stat(fullPath); err != nil {
		return nil, false, err
	}
	if info.Size() < (offset + int64(length)) {
		return nil, false, errors.New(fmt.Sprintf("no found!"))
	} else {
		if data, err = s.util.ReadFileByOffSet(fullPath, offset, length); err != nil {
			return nil, false, err
		}
		return data, false, err
	}

}

func (s *Server) DownloadSmallFileByURI(w http.ResponseWriter, r *http.Request) (bool, error) {
	var (
		err           error
		data          []byte
		isDownload    bool
		imgWidth      int
		imgHeight     int
		width, height string
		notFound      bool
	)
	r.ParseForm()
	isDownload = true
	if r.FormValue("download") == "" {
		isDownload = Config().DefaultDownload
	}
	if r.FormValue("download") == "0" {
		isDownload = false
	}
	width = r.FormValue("width")
	height = r.FormValue("height")
	if imgWidth, err = strconv.Atoi(width); err != nil {
		logrus.Errorf("width error!width=%s", width)
	}
	if imgHeight, err = strconv.Atoi(height); err != nil {
		logrus.Errorf("height error!height=%s", height)
	}
	data, notFound, err = s.GetSmallFileByURI(w, r)
	if data != nil && data[0] == 1 {
		if isDownload {
			s.SetDownloadHeader(w, r)
		}
		if imgWidth != 0 || imgHeight != 0 {
			//todo
			//s.ResizeImageByBytes(w,data[1:],uint(imgWidth),uint(imgHeight))
			return true, nil
		}
		w.Write(data[1:])
		return true, nil
	}
	return false, errors.New("not found!")
}

func (s *Server) DownloadNormalFileByURI(w http.ResponseWriter, r *http.Request) (bool, error) {
	var (
		err                 error
		isDownload          bool
		imgWidth, imgHeight int
		width, height       string
	)
	r.ParseForm()
	isDownload = true
	downloadStr := r.FormValue("download")
	if downloadStr == "" {
		isDownload = Config().DefaultDownload
	} else if downloadStr == "0" {
		isDownload = false
	}
	width = r.FormValue("width")
	height = r.FormValue("height")
	if imgWidth, err = strconv.Atoi(width); err != nil {
		logrus.Errorf("width error!width=%s;err=%+v", width, err)
	}
	if imgHeight, err = strconv.Atoi(height); err != nil {
		logrus.Errorf("width error!height=%s;err=%+v", height, err)
	}

	if isDownload {
		s.SetDownloadHeader(w, r)
	}

	fullPath, _ := s.GetFilePathFromRequest(w, r)
	if imgWidth != 0 || imgHeight != 0 {
		//todo
		//s.ResizeImage(w,fullPath,uint(imgWidth),uint(imgHeight))
		return true, nil
	}
	staticHandler.ServeHTTP(w, r)
	return true, nil
}

func (s *Server) DownloadNotFound(w http.ResponseWriter, r *http.Request) {
	var (
		err                 error
		smallPath, fullPath string
		isDownload          bool
		pathMd5, peer       string
		fileInfo            *FileInfo
	)
	fullPath, smallPath = s.GetFilePathFromRequest(w, r)
	isDownload = true
	downloadStr := r.FormValue("download")
	if downloadStr == "" {
		isDownload = Config().DefaultDownload
	} else if downloadStr == "0" {
		isDownload = false
	}

	if smallPath != "" {
		pathMd5 = s.util.MD5(smallPath)
	} else {
		pathMd5 = s.util.MD5(fullPath)
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

func (s *Server) Download(w http.ResponseWriter, r *http.Request) {
	var (
		err                 error
		ok                  bool
		smallPath, fullPath string
		fi                  os.FileInfo
	)
	//redirect to upload
	if r.RequestURI == "/" || r.RequestURI == "" || r.RequestURI == "/"+Config().Group || r.RequestURI == "/"+Config().Group+"/" {
		//todo
		//s.Index(w,r)
		return
	}

	if ok, err = s.CheckDownloadAuth(w, r); !ok {
		log.Errorf("CheckDownloadAuth error!err=%+v\n", err)
		s.NotPermit(w, r)
		return
	}
	if Config().EnableCrossOrigin {
		s.CrossOrigin(w, r)
	}
	fullPath, smallPath = s.GetFilePathFromRequest(w, r)
	if smallPath == "" {
		if fi, err = os.Stat(fullPath); err != nil {
			s.DownloadNotFound(w, r)
			return
		}
		if !Config().ShowDir && fi.IsDir() {
			w.Write([]byte("list dir deny"))
			return
		}
		s.DownloadNormalFileByURI(w, r)
		return
	}
	if smallPath != "" {
		if ok, err = s.DownloadSmallFileByURI(w, r); !ok {
			s.DownloadNotFound(w, r)
			return
		}
		return
	}
}

func (s *Server) DownloadFileToResponse(url string, w http.ResponseWriter, r *http.Request) {
	var (
		err  error
		req  *httplib.BeegoHTTPRequest
		resp *http.Response
	)
	req = httplib.Get(url)
	req.SetTimeout(time.Second*20, time.Second*600)
	if resp, err = req.DoRequest(); err != nil {
		logrus.Errorf("DoRequest error!err=%+v", err)
		return
	}
	defer resp.Body.Close()
	if _, err = io.Copy(w, resp.Body); err != nil {
		logrus.Errorf("Copy error!err=%+v\n", err)
	}
}

func (s *Server) ResizeImageByBytes(w http.ResponseWriter, data []byte, width, height uint) {
	var (
		img     image.Image
		err     error
		imgType string
	)
	reader := bytes.NewReader(data)
	if img, imgType, err = image.Decode(reader); err != nil {
		logrus.Errorf("decode error!err=%+v", err)
		return
	}
	img = resize.Resize(width, height, img, resize.Lanczos3)
	if imgType == "jpg" || imgType == "jpeg" {
		jpeg.Encode(w, img, nil)
	} else if imgType == "png" {
		png.Encode(w, img)
	} else {
		w.Write(data)
	}
}

func (s *Server) ResizeImage(w http.ResponseWriter, fullPath string, width, height uint) {
	var (
		img     image.Image
		err     error
		imgType string
		file    *os.File
	)
	if file, err = os.Open(fullPath); err != nil {
		logrus.Errorf("open file error!fullPath=%s;err=%+v;", fullPath, err)
		return
	}
	if img, imgType, err = image.Decode(file); err != nil {
		logrus.Errorf("image decode error!err=%+v", err)
		return
	}
	file.Close()

	img = resize.Resize(width, height, img, resize.Lanczos3)
	if imgType == "jpg" || imgType == "jpeg" {
		jpeg.Encode(w, img, nil)
	} else if imgType == "png" {
		png.Encode(w, img)
	} else {
		file.Seek(0, 0)
		io.Copy(w, file)
	}
}

func (s *Server) GetServerURI(r *http.Request) string {
	return fmt.Sprintf("http://%s/", r.Host)
}

func (s *Server) CheckFileAndSendToPeer(date string, fileName string, isForceUpload bool) {
	logrus.Infof("date=%+v;fileName=%+v;isForceUpload=%+v;", date, fileName, isForceUpload)
	var (
		md5set   mapset.Set
		err      error
		md5s     []interface{}
		fileInfo *FileInfo
	)
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			logrus.Errorf("CheckFileAndSendToPeer;re=%+v;buffer=%+v;", re, string(buffer))
		}
	}()
	//todo
	/*	if md5set,err=s.GetMd5sByDate(date,fileName);err!=nil{
		log.Errorf("GetMd5sByDate error!date=%s;fileName=%s;err=%+v;",date,fileName,err)
		return
	}*/
	md5s = md5set.ToSlice()
	for _, md5 := range md5s {
		if md5 == nil {
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

func (s *Server) postFileToPeer(fileInfo *FileInfo) {
	var (
		err                                    error
		peer, fileName, postURL, result, fpath string
		info                                   *FileInfo
		fi                                     os.FileInfo
		i                                      int
		data                                   []byte
	)
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			logrus.Errorf("postFileToPeer;re=%+v;buffer=%+v;", re, string(buffer))
		}
	}()

	for i, peer = range Config().Peers {
		if fileInfo.Peers == nil {
			fileInfo.Peers = []string{}
		}
		if s.util.Contains(peer, fileInfo.Peers) {
			continue
		}
		fileName = fileInfo.Name
		if fileInfo.ReName != "" {
			fileName = fileInfo.ReName
			if fileInfo.OffSet != -1 {
				fileName = strings.Split(fileInfo.ReName, ",")[0]
			}
		}
		fpath = DOCKER_DIR + fileInfo.Path + "/" + fileName
		if !s.util.FileExists(fpath) {
			logrus.Warnf("file %s not found!", fpath)
			continue
		} else {
			if fileInfo.Size == 0 {
				if fi, err = os.Stat(fpath); err != nil {
					logrus.Errorf("os.Stat error!fpath=%s;err=%s;", fpath, err)
				} else {
					fileInfo.Size = fi.Size()
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
		b := httplib.Post(postURL)
		b.SetTimeout(time.Second*30, time.Second*30)
		if data, err = json.Marshal(fileInfo); err != nil {
			logrus.Errorf("marshal error!err=%+v\n", err)
			return
		}
		b.Param("fileInfo", string(data))
		result, err = b.String()
		if err != nil {
			if fileInfo.retry <= Config().RetryCount {
				fileInfo.retry = fileInfo.retry + 1
				//todo
				//s.AppendToQueue(fileInfo)
			}
			logrus.Errorf("http requet error!err=%%+v;path=%s;", err, fileInfo.Path+"/"+fileInfo.Name)
		}
		if !strings.HasPrefix(result, "http://") {
			logrus.Infof("result=%s;", result)
			if !s.util.Contains(peer, fileInfo.Peers) {
				fileInfo.Peers = append(fileInfo.Peers, peer)
				//todo
				/*			if _,err=s.SaveFileInfoToLevelDB(fileInfo.Md5,fileInfo,s.ldb);err!=nil{
							logrus.Errorf("SaveFileInfoToLevelDB error!err=%+v",err)
						}*/
			}
		}

		if err != nil {
			logrus.Errorf("err=%+v", err)
		}

	} //for
}

func (s *Server) SaveFileMd5Log(fileInfo *FileInfo, fileName string) {
	for len(s.queueFileLog)+len(s.queueFileLog)/10 > CONST_QUEUE_SIZE {
		time.Sleep(time.Second * 1)
	}
	info := *fileInfo
	s.queueFileLog <- &FileLog{FileInfo: &info, FileName: fileName}
}

func (s *Server) saveFileMd5Log(fileInfo *FileInfo, fileName string) {
	var (
		err                                         error
		outname, logDate, fullPath, md5Path, logKey string
		ok                                          bool
	)
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			logrus.Errorf("saveFileMd5Log;re=%+v;buffer=%+v;", re, string(buffer))
		}
	}()
	if fileInfo == nil || fileInfo.Md5 == "" || fileName == "" {
		logrus.Warnf("saveFileMd5Log!fileInfo=%+v;fileName=%s;", fileInfo, fileName)
		return
	}
	logDate = s.util.GetDayFromTimeStamp(fileInfo.TimeStamp)
	outname = fileInfo.Name
	if fileInfo.ReName != "" {
		outname = fileInfo.ReName
	}
	fullPath = fileInfo.Path + "/" + outname
	logKey = fmt.Sprintf("%s_%s_%s", logDate, fileName, fileInfo.Md5)
	if fileName == CONST_FILE_Md5_FILE_NAME {
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

func (s *Server) checkPeerFileExist(peer string, md5sum string, fpath string) (*FileInfo, error) {
	var (
		err      error
		fileInfo FileInfo
	)
	//todo
	req := httplib.Post(fmt.Sprintf("%s%s?md5=%s", peer, s.getRequestURI("check_file_exist"), md5sum))
	req.Param("path", fpath)
	req.Param("md5", md5sum)
	req.SetTimeout(time.Second*5, time.Second*10)

	if err = req.ToJSON(&fileInfo); err != nil {
		return &FileInfo{}, err
	}

	if fileInfo.Md5 == "" {
		return &fileInfo, errors.New("not found")
	}
	return &fileInfo, nil
}

func (s *Server) CheckFileExist(w http.ResponseWriter, r *http.Request) {
	var (
		data     []byte
		err      error
		fileInfo *FileInfo
		fpath    string
		fi       os.FileInfo
	)
	r.ParseForm()
	md5sum := r.FormValue("md5")
	fpath = r.FormValue("path")
	//todo
	if fileInfo, err = s.GetFileInfoFromLevelDB(md5sum); fileInfo != nil {
		if fileInfo.OffSet != -1 {
			if data, err = json.Marshal(fileInfo); err != nil {
				logrus.Errorf("marshal error!fileInfo=%+v;err=%+v;", fileInfo, err)
			}
			w.Write(data)
			return
		}
		fpath = DOCKER_DIR + fileInfo.Path + "/" + fileInfo.Name
		if fileInfo.ReName != "" {
			fpath = DOCKER_DIR + fileInfo.Path + "/" + fileInfo.ReName
		}
		if s.util.IsExist(fpath) {
			if data, err = json.Marshal(fileInfo); err == nil {
				w.Write(data)
				return
			} else {
				logrus.Errorf("Marshal error!fileInfo=%+v;err=%+v;", fileInfo, err)
			}
		} else {
			if fileInfo.OffSet == -1 {
				//todo
				//s.RemoveKeyFromLevelDB(md5sum,s.ldb)
			}
		}
	} else {
		if fpath != "" {
			if fi, err = os.Stat(fpath); err == nil {
				sum := s.util.MD5(fpath)
				fileInfo = &FileInfo{
					Path:      path.Dir(fpath),
					Name:      path.Base(fpath),
					Size:      fi.Size(),
					Md5:       sum,
					Peers:     []string{Config().Host},
					OffSet:    -1,
					TimeStamp: fi.ModTime().Unix(),
				}
			}
			data, err = json.Marshal(fileInfo)
			w.Write(data)
			return
		}
	}
	data, _ = json.Marshal(FileInfo{})
	w.Write(data)
}

func (s *Server) Sync(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var jsonResult JsonResult
	jsonResult.Status = "fail"
	//todo
	if !s.IsPeer(r) {
		jsonResult.Message = "client must be in cluster"
		w.Write([]byte(s.util.JsonEncodePretty(jsonResult)))
		return
	}
	date := ""
	force := ""
	inner := ""
	isForceUpload := false
	force = r.FormValue("force")
	date = r.FormValue("date")
	inner = r.FormValue("inner")

	if force == "1" {
		isForceUpload = true
	}
	if inner != "1" {
		for _, peer := range Config().Peers {
			req := httplib.Post(peer + s.getRequestURI("sync"))
			req.Param("force", force)
			req.Param("inner", inner)
			req.Param("date", date)
			if _, err := req.String(); err != nil {
				logrus.Errorf("req error!err=%+v", err)
			}
		} //for
	}
	if date == "" {
		jsonResult.Message = "require params date &force,?date=20181230"
		w.Write([]byte(s.util.JsonEncodePretty(jsonResult)))
		return
	}
	date = strings.Replace(date, ".", "", -1)

	if isForceUpload {
		go s.CheckFileAndSendToPeer(date, CONST_FILE_Md5_FILE_NAME, isForceUpload)
	} else {
		go s.CheckFileAndSendToPeer(date, CONST_Md5_ERROR_FILE_NAME, isForceUpload)
	}
	jsonResult.Status = "ok"
	jsonResult.Message = "jos is running"
	w.Write([]byte(s.util.JsonEncodePretty(jsonResult)))
}

func (s *Server) IsExistFromLevelDB(key string, db *leveldb.DB) (bool, error) {
	return db.Has([]byte(key), nil)
}

func (s *Server) GetFileInfoFromLevelDB(key string) (*FileInfo, error) {
	var (
		err      error
		data     []byte
		fileInfo FileInfo
	)
	if data, err = s.ldb.Get([]byte(key), nil); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &fileInfo); err != nil {
		return nil, err
	}
	return &fileInfo, nil
}

func (s *Server) SaveStat() {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("SaveStatFunc")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	stat := s.statMap.Get()
	if v, ok := stat[CONST_STAT_FILE_COUNT_KEY]; ok {
		switch v.(type) {
		case int64, int32, int, float64, float32:
			if v.(int64) >= 0 {
				if data, err := json.Marshal(stat); err != nil {
					logrus.Errorf("Marshal error!stat=%+v;err=%+v;", stat, err)
				} else {
					s.util.WriteBinFile(CONST_STAT_FILE_NAME, data)
				}
			}
		}
	}

}

func (s *Server) RemoveKeyFromLevelDB(key string, db *leveldb.DB) error {
	return db.Delete([]byte(key), nil)
}

func (s *Server) SaveFileInfoToLevelDB(key string, fileInfo *FileInfo, db *leveldb.DB) (*FileInfo, error) {
	if fileInfo == nil || db == nil {
		return nil, errors.New("fileInfo is null or db is null")
	}
	var (
		data []byte
		err  error
	)
	if data, err = json.Marshal(fileInfo); err != nil {
		return fileInfo, err
	}
	if err = db.Put([]byte(key), data, nil); err != nil {
		return fileInfo, err
	}
	if db == s.ldb {
		logDate := s.util.GetDayFromTimeStamp(fileInfo.TimeStamp)
		logKey := fmt.Sprintf("%s_%s_%s", logDate, CONST_FILE_Md5_FILE_NAME, fileInfo.Md5)
		s.logDB.Put([]byte(logKey), data, nil)
	}
	return fileInfo, nil
}

func isPublicIP(IP *net.IP) bool {
	if IP.IsLoopback() || IP.IsLinkLocalMulticast() || IP.IsLinkLocalUnicast() {
		return false
	}
	if ip4 := IP.To4(); ip4 != nil {
		switch true {
		case ip4[0] == 10 || (ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) || (ip4[0] == 192 && ip4[1] == 168):
			return false
		default:
			return true
		}
	}
	return false
}

func (s *Server) IsPeer(r *http.Request) bool {
	var (
		ip    string
		peer  string
		bflag bool
		cidr  *net.IPNet
		err   error
	)
	ip = s.util.GetClientIp(r)
	clientIP := net.ParseIP(ip)
	if s.util.Contains("0.0.0.0", Config().AdminIps) {
		if isPublicIP(&clientIP) {
			return false
		}
		return true
	}

	if s.util.Contains(ip, Config().AdminIps) {
		return true
	}
	for _, v := range Config().AdminIps {
		if strings.Contains(v, "/") {
			if _, cidr, err = net.ParseCIDR(v); err != nil {
				logrus.Errorf("ParseCIDR error!v=%s;err=%+v", v, err)
				return false
			}
			if cidr.Contains(clientIP) {
				return true
			}
		}
	} //for
	realIp := os.Getenv("GO_FASTDFS_IP")
	if realIp == "" {
		realIp = s.util.GetPulicIP()
	}
	if ip == "127.0.0.1" || ip == realIp {
		return true
	}
	ip = "http://" + ip
	bflag = false
	for _, peer = range Config().Peers {
		if strings.HasPrefix(peer, ip) {
			bflag = true
			break
		}
	}
	return bflag
}

func (s *Server) RecvMd5s(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		md5str   string
		fileInfo *FileInfo
		md5s     []string
	)
	if !s.IsPeer(r) {
		logrus.Warnf("RecvMd5s;ip=%s;", s.util.GetClientIp(r))
		//todo
		//w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}
	r.ParseForm()

	md5str = r.FormValue("md5s")
	md5s = strings.Split(md5str, ",")
	go func(md5s []string) {
		for _, m := range md5s {
			if m != "" {
				if fileInfo, err = s.GetFileInfoFromLevelDB(m); err != nil {
					logrus.Errorf("GetFileInfoFromLevelDB error!m=%s;err=%+v;", m, err)
					continue
				}
				//todo
				//s.AppendToQueue(fileInfo)
			}
		}
	}(md5s)
}

func (s *Server) GetClusterNotPermitMessage(r *http.Request) string {
	return fmt.Sprintf(CONST_MESSAGE_CLUSTER_IP, s.util.GetClientIp(r))
}

func (s *Server) GetMd5sForWeb(w http.ResponseWriter, r *http.Request) {
	var (
		date   string
		err    error
		result mapset.Set
		lines  []string
		md5s   []interface{}
	)
	if !s.IsPeer(r) {
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}
	date = r.FormValue("date")
	//todo
	if result, err = s.GetMd5sByDate(date, CONST_FILE_Md5_FILE_NAME); err != nil {
		logrus.Errorf("GetMd5sByDate error!date=%s;err=%+v;", date, err)
		return
	}

	md5s = result.ToSlice()
	for _, line := range md5s {
		if line != nil && line != "" {
			lines = append(lines, line.(string))
		}
	}
	w.Write([]byte(strings.Join(lines, ",")))
}

func (s *Server) GetMd5File(w http.ResponseWriter, r *http.Request) {
	var (
		date  string
		fpath string
		data  []byte
		err   error
	)
	if !s.IsPeer(r) {
		return
	}
	fpath = DATA_DIR + "/" + date + "/" + CONST_FILE_Md5_FILE_NAME

	if !s.util.FileExists(fpath) {
		w.WriteHeader(404)
		return
	}
	if data, err = ioutil.ReadFile(fpath); err != nil {
		w.WriteHeader(500)
		return
	}
	w.Write(data)
}

func (s *Server) GetMd5sMapByDate(date, fileName string) (*goutil.CommonMap, error) {
	var (
		err                  error
		result               *goutil.CommonMap
		fpath, content, line string
		lines, cols          []string
		data                 []byte
	)
	result = goutil.NewCommonMap(0)
	if fileName == "" {
		fpath = DATA_DIR + "/" + date + "/" + CONST_FILE_Md5_FILE_NAME
	} else {
		fpath = DATA_DIR + "/" + date + "/" + fileName
	}
	if !s.util.FileExists(fpath) {
		return result, errors.New(fmt.Sprintf("fpath %s not found!", fpath))
	}
	if data, err = ioutil.ReadFile(fpath); err != nil {
		return result, err
	}
	content = string(data)
	lines = strings.Split(content, "\n")
	for _, line = range lines {
		cols = strings.Split(line, "|")
		if len(cols) > 2 {
			if _, err = strconv.ParseInt(cols[1], 10, 64); err != nil {
				continue
			}
			result.Add(cols[0])
		}
	} //for
	return result, nil
}

func (s *Server) GetMd5sByDate(date string, fileName string) (mapset.Set, error) {
	var (
		keyPrefix string
		md5set    mapset.Set
		keys      []string
	)
	md5set = mapset.NewSet()
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf("%s_%s_", date, fileName)
	it := server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)), nil)
	for it.Next() {
		keys = strings.Split(string(it.Key()), "_")
		if len(keys) >= 3 {
			md5set.Add(keys[2])
		}
	} //for
	it.Release()
	return md5set, nil
}

func (s *Server) SyncFileInfo(w http.ResponseWriter, r *http.Request) {
	var (
		err         error
		fileInfo    FileInfo
		fileInfoStr string
		fileName    string
	)
	r.ParseForm()
	fileInfoStr = r.FormValue("fileInfo")

	if !s.IsPeer(r) {
		logrus.Errorf("is not peer fileInfo!fileInfo=%+v", fileInfo)
		return
	}
	if err = json.Unmarshal([]byte(fileInfoStr), &fileInfo); err != nil {
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		logrus.Errorf("unmarshal error!err=%+v", err)
		return
	}
	if fileInfo.OffSet == -2 {
		s.SaveFileInfoToLevelDB(fileInfo.Md5, &fileInfo, s.ldb)
	} else {
		s.SaveFileMd5Log(&fileInfo, CONST_Md5_QUEUE_FILE_NAME)
	}
	//todo
	s.AppendToDownloadQueue(&fileInfo)
	fileName = fileInfo.Name

	if fileInfo.ReName != "" {
		fileName = fileInfo.ReName
	}

	p := strings.Replace(fileInfo.Path, STORE_DIR+"/", "", 1)
	downloadUrl := fmt.Sprintf("http://%s/%s", r.Host, Config().Group+"/"+p+"/"+fileName)
	logrus.Infof("SyncFileInfo!downloadUrl=%s;", downloadUrl)
	w.Write([]byte(downloadUrl))
}

func (s *Server) CheckScene(scene string) (bool, error) {

	if len(Config().Scenes) == 0 {
		return true, nil
	}
	var scenes []string
	for _, s := range Config().Scenes {
		scenes = append(scenes, strings.Split(s, ":")[0])
	}
	if !s.util.Contains(scene, scenes) {
		return false, errors.New("not valid scene")
	}
	return true, nil
}

func (s *Server) GetFileInfo(w http.ResponseWriter, r *http.Request) {
	var (
		fpath    string
		md5sum   string
		fileInfo *FileInfo
		err      error
		result   JsonResult
	)
	md5sum = r.FormValue("md5")
	fpath = r.FormValue("path")
	result.Status = "fail"

	if !s.IsPeer(r) {
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}

	md5sum = r.FormValue("md5")
	if fpath != "" {
		fpath = strings.Replace(fpath, "/"+Config().Group+"/", STORE_DIR_NAME+"/", 1)
		md5sum = s.util.MD5(fpath)
	}

	if fileInfo, err = s.GetFileInfoFromLevelDB(md5sum); err != nil {
		logrus.Errorf("GetFileInfoFromLevelDB error!err=%+v", err)
		result.Message = err.Error()
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}

	result.Status = "ok"
	result.Data = fileInfo
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) RemoveFile(w http.ResponseWriter, r *http.Request) {
	var (
		err                                error
		md5sum, fpath, delUrl, inner, name string
		fileInfo                           *FileInfo
		result                             JsonResult
	)
	r.ParseForm()
	md5sum = r.FormValue("md5")
	fpath = r.FormValue("path")
	inner = r.FormValue("inner")
	result.Status = "fail"
	if !s.IsPeer(r) {
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}
	if Config().AuthUrl != "" && !s.CheckAuth(w, r) {
		s.NotPermit(w, r)
		return
	}
	if fpath != "" && md5sum == "" {
		fpath = strings.Replace(fpath, "/"+Config().Group+"/", STORE_DIR_NAME+"/", 1)
		md5sum = s.util.MD5(fpath)
	}
	if inner != "1" {
		for _, peer := range Config().Peers {
			go func(peer string, md5sum string, fileInfo *FileInfo) {
				//todo
				delUrl = fmt.Sprintf("%s%s", peer, s.getRequestURI("delete"))
				req := httplib.Post(delUrl)
				req.Param("md5", md5sum)
				req.Param("inner", "1")
				req.SetTimeout(time.Second*5, time.Second*10)
				if _, err = req.String(); err != nil {
					logrus.Errorf("req error!err=%+v", err)
				}
			}(peer, md5sum, fileInfo)
		} //for
	}

	if len(md5sum) < 32 {
		result.Message = "md5 unvalid"
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	if fileInfo, err = s.GetFileInfoFromLevelDB(md5sum); err != nil {
		result.Message = err.Error()
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}

	if fileInfo.OffSet >= 0 {
		result.Message = "small file delete not support!"
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	name = fileInfo.Name
	if fileInfo.ReName != "" {
		name = fileInfo.ReName
	}
	fpath = fileInfo.Path + "/" + name
	if fileInfo.Path != "" && s.util.FileExists(DOCKER_DIR+fpath) {
		s.SaveFileMd5Log(fileInfo, CONST_REMOVE_Md5_FILE_NAME)
		if err = os.Remove(DOCKER_DIR + fpath); err != nil {
			result.Message = err.Error()
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		} else {
			result.Message = "remove success"
			result.Status = "ok"
			w.Write([]byte(s.util.JsonEncodePretty(result)))
		}
	}
	result.Message = "fail to remove"
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) getRequestURI(action string) string {
	var uri string
	if Config().SupportGroupManage {
		uri = "/" + Config().Group + "/" + action
	} else {
		uri = "/" + action
	}
	return uri
}

func (s *Server) BuildFileResult(fileInfo *FileInfo, r *http.Request) FileResult {
	var (
		outname     string
		fileResult  FileResult
		p           string
		downloadUrl string
		domain      string
		host        string
		protocol    string
	)
	if Config().EnableHttps {
		protocol = "https"
	} else {
		protocol = "http"
	}
	host = strings.Replace(Config().Host, "http://", "", -1)
	if r != nil {
		host = r.Host
	}
	if !strings.HasPrefix(Config().DownloadDomain, "http") {
		if Config().DownloadDomain == "" {
			Config().DownloadDomain = fmt.Sprintf("%s://%s", protocol, host)
		} else {
			Config().DownloadDomain = fmt.Sprintf("%s://%s", protocol, Config().DownloadDomain)
		}
	}

	if Config().DownloadDomain != "" {
		domain = Config().DownloadDomain
	} else {
		domain = fmt.Sprintf("%s://%s", protocol, host)
	}
	outname = fileInfo.Name
	if fileInfo.ReName != "" {
		outname = fileInfo.ReName
	}
	p = strings.Replace(fileInfo.Path, STORE_DIR_NAME+"/", "", 1)
	if Config().SupportGroupManage {
		p = Config().Group + "/" + p + "/" + outname
	} else {
		p = p + "/" + outname
	}
	downloadUrl = fmt.Sprintf("%s://%s/%s", protocol, host, p)

	if Config().DownloadDomain != "" {
		downloadUrl = fmt.Sprintf("%s/%s", Config().DownloadDomain, p)
	}
	fileResult.Url = downloadUrl
	if Config().DefaultDownload {
		fileResult.Url = fmt.Sprintf("%s?name=%s&download=1", downloadUrl, url.PathEscape(outname))
	}
	fileResult.Md5 = fileInfo.Md5
	fileResult.Path = "/" + p
	fileResult.Domain = domain
	fileResult.Scene = fileInfo.Scene
	fileResult.Size = fileInfo.Size
	fileResult.ModTime = fileInfo.TimeStamp
	fileResult.Src = fileResult.Path
	fileResult.Scenes = fileInfo.Scene

	return fileResult
}

func (s *Server) SaveUploadFile(file multipart.File, header *multipart.FileHeader,
	fileInfo *FileInfo, r *http.Request) (*FileInfo, error) {
	var (
		err     error
		outFile *os.File
		folder  string
		fi      os.FileInfo
	)
	defer file.Close()
	_, fileInfo.Name = filepath.Split(header.Filename)
	if len(Config().Extensions) > 0 && !s.util.Contains(path.Ext(fileInfo.Name), Config().Extensions) {
		return fileInfo, errors.New("error file extension mismatch!")
	}
	if Config().RenameFile {
		fileInfo.ReName = s.util.MD5(s.util.GetUUID() + path.Ext(fileInfo.Name))
	}
	folder = time.Now().Format("20060102/15/04")
	if Config().PeerId != "" {
		folder = fmt.Sprintf("folder/%s", Config().PeerId)
	} else {
		folder = fmt.Sprintf("%s/%s", STORE_DIR, folder)
	}
	if fileInfo.Path != "" {
		if strings.HasPrefix(fileInfo.Path, STORE_DIR) {
			folder = fileInfo.Path
		} else {
			folder = STORE_DIR + "/" + fileInfo.Path
		}
	}

	if !s.util.FileExists(folder) {
		if err = os.MkdirAll(folder, 0775); err != nil {
			logrus.Errorf("mkdir error!folder=%s", folder)
		}
	}
	outPath := fmt.Sprintf("%s/%s", folder, fileInfo.Name)
	if fileInfo.ReName != "" {
		outPath = fmt.Sprintf("%s/%s", folder, fileInfo.ReName)
	}
	if s.util.FileExists(outPath) && Config().EnableDistinctFile {
		for i := 0; i < 10000; i++ {
			outPath = fmt.Sprintf("%s/%d_%s", folder, i, filepath.Base(header.Filename))
			fileInfo.Name = fmt.Sprintf("%d_%s", i, header.Filename)
			if !s.util.FileExists(outPath) {
				break
			}
		} //for
	}

	logrus.Infof("upload:%s", outPath)
	if outFile, err = os.Create(outPath); err != nil {
		return fileInfo, err
	}
	defer outFile.Close()
	if fi, err = outFile.Stat(); err != nil {
		logrus.Errorf("outFile stat error!err=%+v", err)
		return fileInfo, errors.New(fmt.Sprintf("(error) file fail!err=%+v", err))
	} else {
		fileInfo.Size = fi.Size()
	}
	if fi.Size() != header.Size {
		return fileInfo, errors.New(fmt.Sprintf("(error) file incomplete!fi.size=%%d;header.size=%d;", fi.Size(), header.Size))
	}
	if Config().EnableDistinctFile {
		fileInfo.Md5 = s.util.GetFileSum(outFile, Config().FileSumArithmetic)
	} else {
		fileInfo.Md5 = s.util.MD5(s.GetFilePathByInfo(fileInfo, false))
	}
	fileInfo.Path = strings.Replace(folder, DOCKER_DIR, "", 1)
	fileInfo.Peers = append(fileInfo.Peers, s.host)
	return fileInfo, nil
}

func (s *Server) Upload(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		fn     string
		folder string
		fpTmp  *os.File
		//fpBody *os.File
	)
	if r.Method == http.MethodGet {
		//todo
		//s.upload(w,r);
		return
	}
	folder = STORE_DIR + "/_tmp/" + time.Now().Format("20060102")
	if s.util.FileExists(folder) {
		if err = os.MkdirAll(folder, 0777); err != nil {
			logrus.Errorf("MkdirAll error!err=%+v", err)
		}
	}
	fn = folder + "/" + s.util.GetUUID()
	if fpTmp, err = os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0777); err != nil {
		logrus.Errorf("open file error!err=%+v", err)
		w.Write([]byte(err.Error()))
		return
	}
	defer fpTmp.Close()
	if _, err = io.Copy(fpTmp, r.Body); err != nil {
		logrus.Errorf("copy error!err=%+v", err)
		w.Write([]byte(err.Error()))
		return
	}
	var fpBody *os.File
	if fpBody, err = os.OpenFile(fn, os.O_RDONLY, 0); err != nil {
		logrus.Errorf("open error!err=%+v", err)
		w.Write([]byte(err.Error()))
		return
	}
	r.Body = fpBody

	defer func() {
		if err = fpBody.Close(); err != nil {
			logrus.Errorf("fp Body close error!err=%+v", err)
		}
		if err = os.Remove(fn); err != nil {
			logrus.Errorf("remove fn!err=%+v", err)
		}
	}()
	done := make(chan bool, 1)
	s.queueUpload <- WrapReqResp{
		w:    &w,
		r:    r,
		done: done,
	}
	<-done
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	var (
		err                                        error
		ok                                         bool
		md5sum, fileName, scene, output, code, msg string
		fileInfo                                   FileInfo
		uploadFile                                 multipart.File
		uploadHeader                               *multipart.FileHeader
		fileResult                                 FileResult
		result                                     JsonResult
		data                                       []byte
		secret                                     interface{}
	)
	output = r.FormValue("output")
	if Config().EnableCrossOrigin {
		s.CrossOrigin(w, r)
		if r.Method == http.MethodOptions {
			return
		}
	}
	result.Status = "fail"
	if Config().AuthUrl != "" && !s.CheckAuth(w, r) {
		msg = "auth fail"
		logrus.Warnf("msg=%s;r.Form=%+v;", msg, r.Form)
		s.NotPermit(w, r)
		result.Message = msg
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}

	if r.Method == http.MethodPost {
		md5sum = r.FormValue("md5")
		fileName = r.FormValue("filename")
		output = r.FormValue("output")
		if Config().ReadOnly {
			msg = "(error) readonly"
			result.Message = msg
			logrus.Warnf("msg=%s", msg)
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}

		if Config().EnableCustomPath {
			fileInfo.Path = r.FormValue("path")
			fileInfo.Path = strings.Trim(fileInfo.Path, "/")
		}
		scene = r.FormValue("scene")
		code = r.FormValue("code")
		if scene == "" {
			scene = r.FormValue("scenes")
		}
		if Config().EnableGoogleAuth && scene != "" {
			if secret, ok = s.sceneMap.GetValue(scene); ok {
				//todo
				if !s.VerifyGoogleCode(secret.(string), code, int64(Config().DownloadTokenExpire/30)) {
					s.NotPermit(w, r)
					result.Message = "invalid request;google code error!"
					logrus.Errorf("google auth error!msg=%S;", result.Message)
					w.Write([]byte(s.util.JsonEncodePretty(result)))
					return
				}
			}
		}
		fileInfo.Md5 = md5sum
		fileInfo.ReName = fileName
		fileInfo.OffSet = -1
		if uploadFile, uploadHeader, err = r.FormFile("file"); err != nil {
			logrus.Errorf("upload error!err=%+v", err)
			result.Message = err.Error()
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		fileInfo.Peers = []string{}
		fileInfo.TimeStamp = time.Now().Unix()
		if scene == "" {
			scene = Config().DefaultScene
		}
		if output == "" {
			output = "text"
		}
		if !s.util.Contains(output, []string{"json", "text", "json2"}) {
			msg = "output just support json or text or json2"
			result.Message = msg
			logrus.Warnf("output unknown!output=%s", output)
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		fileInfo.Scene = scene
		if _, err = s.CheckScene(scene); err != nil {
			result.Message = err.Error()
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			logrus.Errorf("CheckScene error!scene=%s;err=%+v;", scene, err)
			return
		}
		if _, err = s.SaveUploadFile(uploadFile, uploadHeader, &fileInfo, r); err != nil {
			result.Message = err.Error()
			logrus.Errorf("err=%+v", err)
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		if Config().EnableDistinctFile {
			if v, _ := s.GetFileInfoFromLevelDB(fileInfo.Md5); v != nil && v.Md5 != "" {
				fileResult = s.BuildFileResult(v, r)
				if s.GetFilePathByInfo(&fileInfo, false) != s.GetFilePathByInfo(v, false) {
					os.Remove(s.GetFilePathByInfo(&fileInfo, false))
				}
				if output == "json" || output == "json2" {
					if output == "json2" {
						result.Data = fileResult
						result.Status = "ok"
						w.Write([]byte(s.util.JsonEncodePretty(result)))
						return
					}
					w.Write([]byte(s.util.JsonEncodePretty(fileResult)))
				} else {
					w.Write([]byte(fileResult.Url))
				}
				return
			}
		}
		if fileInfo.Md5 == "" {
			result.Message = " fileInfo.Md5 is null "
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			logrus.Errorf("fileInfo.md5 empty!")
			return
		}
		if !Config().EnableDistinctFile {
			fileInfo.Md5 = s.util.MD5(s.GetFilePathByInfo(&fileInfo, false))
		}
		if Config().EnableMergeSmallFile && fileInfo.Size < CONST_SMALL_FILE_SIZE {
			//todo
			if err = s.SaveSmallFile(&fileInfo); err != nil {
				result.Message = err.Error()
				w.Write([]byte(s.util.JsonEncodePretty(result)))
				logrus.Errorf("SaveSmallFile error!err=%+v", err)
				return
			}
		}
		s.saveFileMd5Log(&fileInfo, CONST_FILE_Md5_FILE_NAME)
		go s.postFileToPeer(&fileInfo)
		if fileInfo.Size <= 0 {
			result.Message = "file size is zero"
			logrus.Errorf("err_msg=%s", result.Message)
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		fileResult = s.BuildFileResult(&fileInfo, r)

		if output == "json2" {
			result.Data = fileResult
			result.Status = "ok"
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		} else if output == "json" {
			w.Write([]byte(s.util.JsonEncodePretty(fileResult)))
		} else {
			w.Write([]byte(fileResult.Url))
		}
		return
	} else {
		md5sum = r.FormValue("md5")
		output = r.FormValue("output")
		if md5sum == "" {
			result.Message = "(error) if you want to upload fast md5 is require,and if you want to upload file,you must use post method"
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			logrus.Errorf("md5sum null")
			return
		}
		if v, _ := s.GetFileInfoFromLevelDB(md5sum); v != nil && v.Md5 != "" {
			fileResult = s.BuildFileResult(v, r)
			result.Data = fileResult
			result.Status = "ok"
		}
		if output == "json" || output == "json2" {
			if data, err = json.Marshal(fileResult); err != nil {
				result.Message = err.Error()
				w.Write([]byte(s.util.JsonEncodePretty(result)))
				logrus.Errorf("marshal error!err=%+v", err)
				return
			}
			if output == "json2" {
				w.Write([]byte(s.util.JsonEncodePretty(result)))
				return
			}
			w.Write(data)
		} else {
			w.Write([]byte(fileResult.Url))
		}
	}
}

func (s *Server) SaveSmallFile(fileInfo *FileInfo) error {
	var (
		err                                                  error
		fileName, fpath, largeDir, destPath, reName, fileExt string
		srcFile                                              *os.File
		destFile                                             *os.File
	)
	fileName = fileInfo.Name
	fileExt = path.Ext(fileName)
	if fileInfo.ReName != "" {
		fileName = fileInfo.ReName
	}
	fpath = DOCKER_DIR + fileInfo.Path + "/" + fileName
	largeDir = LARGE_DIR + "/" + Config().PeerId

	if !s.util.FileExists(largeDir) {
		os.MkdirAll(largeDir, 0775)
	}
	reName = fmt.Sprintf("%d", s.util.RandInt(100, 300))
	destPath = largeDir + "/" + reName
	s.lockMap.LockKey(destPath)
	defer s.lockMap.UnLockKey(destPath)
	if s.util.FileExists(fpath) {
		if srcFile, err = os.OpenFile(fpath, os.O_CREATE|os.O_RDONLY, 06666); err != nil {
			return err
		}
		defer func() {
			os.Remove(fpath)
			srcFile.Close()
		}()
		if destFile, err = os.OpenFile(destPath, os.O_CREATE|os.O_RDWR, 06666); err != nil {
			return err
		}
		defer destFile.Close()
		fileInfo.OffSet, err = destFile.Seek(0, 2)
		if _, err = destFile.Write([]byte("1")); err != nil {
			return err
		}
		fileInfo.OffSet--
		fileInfo.Size++
		fileInfo.ReName = fmt.Sprintf("%s,%d,%d,%s", reName, fileInfo.OffSet, fileInfo.Size, fileExt)
		if _, err = io.Copy(destFile, srcFile); err != nil {
			return err
		}
		fileInfo.Path = strings.Replace(largeDir, DOCKER_DIR, "", 1)
	}
	return nil
}

func (s *Server) SendToMail(to, subject, body, mailType string) error {
	host := Config().Mail.Host
	user := Config().Mail.User
	password := Config().Mail.Password
	hp := strings.Split(host, ":")
	auth := smtp.PlainAuth("", user, password, hp[0])

	contentType := "Content-Type: text/plain" + "; charset=UTF-8"
	if mailType == "html" {
		contentType = "Content-Type: text/" + mailType + "; charset=UTF-8"
	}
	msg := []byte("To: " + to + "\r\nFrom: " + user + ">\r\nSubject: " + "\r\n" + contentType + "\r\n\r\n" + body)
	recvs := strings.Split(to, ";")
	return smtp.SendMail(host, auth, user, recvs, msg)
}

func (s *Server) BenchMark(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	batch := new(leveldb.Batch)
	n := 100000000
	for i := 0; i < n; i++ {
		f := FileInfo{
			Peers: []string{"http://192.168.0.1", "http://192.168.2.5"},
			Path:  "20190201/19/02",
		}
		md5str := s.util.MD5(strconv.Itoa(i))
		f.Name = md5str
		f.Md5 = md5str
		if data, err := json.Marshal(&f); err == nil {
			batch.Put([]byte(md5str), data)
		}
		if i%10000 == 0 {
			if batch.Len() > 0 {
				server.ldb.Write(batch, nil)
				batch.Reset()
			}
			logrus.Infof("i=%d;since_seconds=%d;", i, time.Since(t).Seconds())
		}
	}

	durationStr := time.Since(t).String()
	s.util.WriteFile("time.txt", durationStr)
	logrus.Infof("durationStr=%s", durationStr)
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
	if inner != "1" {
		for _, peer := range Config().Peers {
			req := httplib.Post(peer + s.getRequestURI("repair_stat"))
			req.Param("inner", inner)
			req.Param("date", date)
			if _, err := req.String(); err != nil {
				logrus.Errorf("post error!err=%+v", err)
			}
		}
	}
	result.Data = s.RepairStatByDate(date)
	result.Status = "ok"
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) Stat(w http.ResponseWriter, r *http.Request) {
	var (
		result            JsonResult
		inner, echart     string
		category          []string
		barCount, barSize []int64
		dataMap           map[string]interface{}
	)

	if !s.IsPeer(r) {
		result.Message = s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}

	r.ParseForm()
	inner = r.FormValue("inner")
	echart = r.FormValue("echart")
	//todo
	data := s.GetStat()
	result.Status = "ok"
	result.Data = data

	if echart == "1" {
		dataMap = make(map[string]interface{}, 3)

		for _, v := range data {
			barCount = append(barCount, v.FileCount)
			barSize = append(barSize, v.TotalSize)
			category = append(category, v.Date)
		}
		dataMap["barCount"] = barCount
		dataMap["barSize"] = barSize
		dataMap["category"] = category
		result.Data = dataMap
	}

	if inner == "1" {
		w.Write([]byte(s.util.JsonEncodePretty(data)))
	} else {
		w.Write([]byte(s.util.JsonEncodePretty(result)))
	}
}

func (s *Server) GetStat() []StatDateFileInfo {
	var (
		min, max, i int64
		err         error
		rows        []StatDateFileInfo
		total       StatDateFileInfo
	)
	min = 20190101
	max = 20190101
	for k := range s.statMap.Get() {
		ks := strings.Split(k, "_")
		if len(ks) == 2 {
			if i, err = strconv.ParseInt(ks[0], 10, 64); err != nil {
				continue
			}
			if i > max {
				i = max
			}
			if i < min {
				i = min
			}
		}
	} //for
	for i := min; i <= max; i++ {
		str := fmt.Sprintf("%d", i)

		if v, ok := s.statMap.GetValue(str + "_" + CONST_STAT_FILE_COUNT_KEY); ok {
			info := StatDateFileInfo{
				Date: str,
			}
			switch v.(type) {
			case int64:
				info.TotalSize = v.(int64)
				total.TotalSize += v.(int64)
			}

			if v, ok := s.statMap.GetValue(str + "_" + CONST_STAT_FILE_COUNT_KEY); ok {
				switch v.(type) {
				case int64:
					info.FileCount = v.(int64)
					total.FileCount += v.(int64)
				}
			}
			rows = append(rows, info)
		}
	}
	total.Date = "all"
	rows = append(rows, total)
	return rows
}

func (s *Server) RegisterExit() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		for sig := range c {
			switch sig {
			case syscall.SIGHUP, syscall.SIGINFO, syscall.SIGTERM, syscall.SIGQUIT:
				s.ldb.Close()
				logrus.Infof("sig exit!sig=%+v", sig)
				os.Exit(1)
			}
		}
	}()
}

func (s *Server) AppendToQueue(fileInfo *FileInfo) {
	for (len(s.queueToPeers) + CONST_QUEUE_SIZE/10) > CONST_QUEUE_SIZE {
		time.Sleep(time.Millisecond * 50)
	}
	s.queueToPeers <- *fileInfo
}

func (s *Server) AppendToDownloadQueue(fileInfo *FileInfo) {
	for (len(s.queueFromPeers) + CONST_QUEUE_SIZE/10) > CONST_QUEUE_SIZE {
		time.Sleep(time.Millisecond * 50)
	}
	s.queueFromPeers <- *fileInfo
}

func (s *Server) ConsumerDownload() {
	for i := 0; i < Config().SyncWorker; i++ {
		go func() {
			for {
				fileInfo := <-s.queueFromPeers
				if len(fileInfo.Peers) <= 0 {
					logrus.Warnf("Peer is null!fileInfo=%+v", fileInfo)
					continue
				}
				for _, peer := range fileInfo.Peers {
					if strings.Contains(peer, "127.0.0.1") {
						logrus.Warnf("sync error with 127.0.0.1!fileInfo=%+v", fileInfo)
						continue
					}
					if peer != s.host {
						s.DownloadFromPeer(peer, &fileInfo)
						break
					}
				}

			}
		}()
	}
}

func (s *Server) RemoveDownloading() {
	go func() {
		for {
			it := s.ldb.NewIterator(util.BytesPrefix([]byte("downloading_")), nil)
			for it.Next() {
				key := it.Key()
				keys := strings.Split(string(key), "_")
				if len(keys) == 3 {
					if t, err := strconv.ParseInt(keys[1], 10, 64); err == nil && (time.Now().Unix()-t > 600) {
						os.Remove(DOCKER_DIR + keys[2])
					}
				}
			}
			it.Release()
			time.Sleep(time.Minute * 3)
		}
	}()
}

func (s *Server) ConsumerLog() {
	go func() {
		for {
			fileLog := <-s.queueFileLog
			s.saveFileMd5Log(fileLog.FileInfo, fileLog.FileName)
		}
	}()
}

func (s *Server) LoadSearchDict() {
	go func() {
		logrus.Infof("LoadSearchDict ... ")
		var (
			f   *os.File = nil
			err error
		)
		if f, err = os.OpenFile(CONST_SERACH_FILE_NAME, os.O_RDONLY, 0); err != nil {
			logrus.Errorf("open file error!CONST_SERACH_FILE_NAME=%s;err=%+v;", CONST_SERACH_FILE_NAME, err)
			return
		}
		defer f.Close()
		r := bufio.NewReader(f)
		for {
			line, isPrefix, err := r.ReadLine()
			for isPrefix && err == nil {
				kvs := strings.Split(string(line), "\t")
				if len(kvs) == 2 {
					s.searchMap.Put(kvs[0], kvs[1])
				}
			}

		}
		logrus.Infof("finish load search dict!")
	}()
}

func (s *Server) SaveSearchDict() {
	var (
		err        error
		fp         *os.File
		searchDict map[string]interface{}
		k          string
		v          interface{}
	)
	s.lockMap.LockKey(CONST_SEARCH_FILE_NAME)
	defer s.lockMap.UnLockKey(CONST_SEARCH_FILE_NAME)
	searchDict = s.searchMap.Get()
	if fp, err = os.OpenFile(CONST_SEARCH_FILE_NAME, os.O_RDWR, 0755); err != nil {
		logrus.Errorf("open file error!err=%+v", err)
		return
	}

	defer fp.Close()
	for k, v = range searchDict {
		fp.WriteString(fmt.Sprintf("%s\t%s", k, v.(string)))
	}
}

func (s *Server) ConsumerPostToPeer() {
	for i := 0; i < Config().SyncWorker; i++ {
		go func() {
			for {
				fileInfo := <-s.queueToPeers
				s.postFileToPeer(&fileInfo)
			}
		}()
	}
}

func (s *Server) ConsumerUpload() {
	for i := 0; i < Config().UploadWorker; i++ {
		go func() {
			for {
				wr := <-s.queueUpload
				s.upload(*wr.w, wr.r)
				s.rtMap.AddCountInt64(CONST_UPLOAD_COUNTER_KEY, wr.r.ContentLength)

				if v, ok := s.rtMap.GetValue(CONST_UPLOAD_COUNTER_KEY); ok {
					if v.(int64) > (1 << 30) {
						var _v int64 = 0
						s.rtMap.Put(CONST_UPLOAD_COUNTER_KEY, _v)
						debug.FreeOSMemory()
					}
				}
				wr.done <- true
			}
		}()
	}
}

func (s *Server) updateInAutoRepairFunc(peer string, dateStat StatDateFileInfo) {
	//远程拉来数据;
	req := httplib.Get(fmt.Sprintf("%s%s?date=%s&force=%s", peer, s.getRequestURI("sync"), dateStat.Date, "1"))
	req.SetTimeout(time.Second*5, time.Second*5)

	if _, err := req.String(); err != nil {
		logrus.Errorf("request error!err=%+v", err)
	}
	logrus.Infof(fmt.Sprintf("sync file from %s date %s", peer, dateStat.Date))
}

func (s *Server) autoRepairFunc(forceRepair bool) {
	var (
		dateStats                           []StatDateFileInfo
		err                                 error
		countKey, md5s                      string
		localSet, remoteSet, allSet, tmpSet mapset.Set
		fileInfo                            *FileInfo
	)
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("autoRepairFunc")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()

	for _, peer := range Config().Peers {
		req := httplib.Post(fmt.Sprintf("%s%s", peer, s.getRequestURI("stat")))
		req.Param("inner", "1")
		req.SetTimeout(time.Second*5, time.Second*15)
		if err = req.ToJSON(&dateStats); err != nil {
			logrus.Errorf("req error!err=%+v", err)
			continue
		}

		for _, dateStat := range dateStats {
			if dateStat.Date == "all" {
				continue
			}

			countKey = dateStat.Date + "_" + CONST_STAT_FILE_COUNT_KEY
			if v, ok := s.statMap.GetValue(countKey); ok {
				switch v.(type) {
				case int64:
					if v.(int64) == dateStat.FileCount || forceRepair {
						//不相等,找差异;
						req := httplib.Post(fmt.Sprintf("%s%s", peer, s.getRequestURI("get_md5s_by_date")))
						req.SetTimeout(time.Second*15, time.Second*60)
						req.Param("date", dateStat.Date)
						if md5s, err = req.String(); err != nil {
							continue
						}
						if localSet, err = s.GetMd5sByDate(dateStat.Date, CONST_FILE_Md5_FILE_NAME); err != nil {
							logrus.Errorf("GetMd5sMapByDate error!date=%s;err=%+v;", dateStat.Date, err)
							continue
						}
						remoteSet = s.util.StrToMapSet(md5s, ",")
						allSet = localSet.Union(remoteSet)
						md5s = s.util.MapSetToStr(allSet.Difference(localSet), ",")

						req = httplib.Post(fmt.Sprintf("%s%s", peer, s.getRequestURI("receive_md5s")))
						req.SetTimeout(time.Second*15, time.Second*60)
						req.Param("md5s", md5s)
						req.String()
						tmpSet = allSet.Difference(remoteSet)
						for v := range tmpSet.Iter() {
							if v != nil {
								if fileInfo, err = s.GetFileInfoFromLevelDB(v.(string)); err != nil {
									logrus.Errorf("GetFileInfoFromLevelDB error!v=%+v;err=%+v", v, err)
									continue
								}
								s.AppendToQueue(fileInfo)
							}
						} //for
					}
				}
			} else {
				s.updateInAutoRepairFunc(peer, dateStat)
			}
		} //for
	} //for
}

func (s *Server) AutoRepair(forceRepair bool) {
	if s.lockMap.IsLock("AutoRepair") {
		logrus.Warnf("AutoRepair has been locked!")
		return
	}
	s.lockMap.LockKey("AutoRepair")
	defer s.lockMap.UnLockKey("AutoRepair")
	//AutoRepairFunc
	s.autoRepairFunc(forceRepair)
}

func (s *Server) CleanLogLevelDBByDate(date string, fileName string) {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("autoRepairFunc")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var (
		err       error
		keyPrefix string
		keys      mapset.Set
	)
	keys = mapset.NewSet()
	keyPrefix = fmt.Sprintf("%s_%s_", date, fileName)
	it := server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)), nil)
	defer it.Release()
	for it.Next() {
		keys.Add(string(it.Value()))
	}
	for key := range keys.Iter() {
		if err = s.RemoveKeyFromLevelDB(key.(string), s.logDB); err != nil {
			logrus.Errorf("RemoveKeyFromLevelDB error!err=%+v", err)
		}
	}
}

func (s *Server) CleanAndBackup() {
	go func() {
		for {
			time.Sleep(time.Minute * 2)
			var (
				fileNames []string
				yesterday string
			)
			if s.curDate != s.util.GetToDay() {
				fileNames = []string{CONST_Md5_QUEUE_FILE_NAME, CONST_Md5_ERROR_FILE_NAME, CONST_REMOVE_Md5_FILE_NAME}
				yesterday = s.util.GetDayFromTimeStamp(time.Now().AddDate(0, 0, -1).Unix())
				for _, fileName := range fileNames {
					s.CleanLogLevelDBByDate(yesterday, FileName)
				}
				s.BackUpMetaDataByDate(yesterday)
				s.curDate = s.util.GetToDay()
			}
		}
	}()
}

func (s *Server) LoadFileInfoByDate(date string, fileName string) (mapset.Set, error) {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("autoRepairFunc")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var (
		err       error
		keyPrefix string
		fileInfos mapset.Set
	)
	fileInfos = mapset.NewSet()
	keyPrefix = fmt.Sprintf("%s_%s_", date, fileName)
	it := server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)), nil)
	defer it.Release()
	for it.Next() {
		var fileInfo FileInfo
		if err = json.Unmarshal(it.Value(), &fileInfo); err != nil {
			logrus.Warnf("umarshal error!v=%+v", it.Value())
			continue
		}
		fileInfos.Add(&fileInfo)
	}
	return fileInfos, nil
}

func (s *Server) LoadQueueSendToPeer() {
	if queue, err := s.LoadFileInfoByDate(s.util.GetToDay(), CONST_Md5_QUEUE_FILE_NAME); err != nil {
		logrus.Errorf("LoadFileInfoByDate error!date=%+v", s.util.GetToDay())
	} else {
		for fileInfo := range queue.Iter() {
			s.AppendToDownloadQueue(fileInfo.(*FileInfo))
		}
	}
}

func (s *Server) checkInClusterStatus() {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("CheckClusterStatus")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var (
		status        JsonResult
		err           error
		subject, body string
		req           *httplib.BeegoHTTPRequest
		data          []byte
	)
	for _, peer := range Config().Peers {
		req = httplib.Get(fmt.Sprintf("%s%s", peer, s.getRequestURI("status")))
		req.SetTimeout(time.Second*5, time.Second*5)
		err = req.ToJSON(&status)
		if err != nil || status.Status != "ok" {
			for _, to := range Config().AlarmReceivers {
				subject = "fastdfs server error!"
				if err != nil {
					body = fmt.Sprintf("%s\nserver:%s\nerror:\n%s", subject, peer, err.Error())
				} else {
					body = fmt.Sprintf("%s\nserver:%s\n", subject, peer)
				}
				if err = s.SendToMail(to, subject, body, "text"); err != nil {
					logrus.Errorf("sendToMail error!to=%s;err=%+v;", to, err)
				}
			} //for AlarmReceivers;
			if Config().AlarmUrl != "" {
				req = httplib.Post(Config().AlarmUrl)
				req.SetTimeout(time.Second*10, time.Second*10)
				req.Param("message", body)
				req.Param("subject", subject)
				if _, err = req.String(); err != nil {
					logrus.Errorf("req error!alarmUrl=%s;err=%+v;", Config().AlarmUrl, err)
				}
			}
			logrus.Errorf("status error!err=%+v", err)
		} else { //if err!=nil||status.Status!="ok"
			var statusMap map[string]interface{}
			if data, err = json.Marshal(status.Data); err != nil {
				logrus.Errorf("Marshal error!err=%+v", err)
				return
			}
			if err = json.Unmarshal(data, &statusMap); err != nil {
				logrus.Errorf("Unmarshal error!err=%+v", err)
				return
			}
			if v, ok := statusMap["Fs.PeerId"]; ok && v == Config().PeerId {
				logrus.Errorf(fmt.Sprintf("PeerId is conflict:%s", v))
			}
			if v, ok := statusMap["Fs.Local"]; ok && v == Config().Host {
				logrus.Errorf(fmt.Sprintf("Host is conflict:%s", v))
			}
		}
	}
}

func (s *Server) CheckClusterStatus() {
	s.checkInClusterStatus()
	go func() {
		time.Sleep(time.Minute * 3)
		s.checkInClusterStatus()
	}()
}

func (s *Server) RepairFileInfo(w http.ResponseWriter, r *http.Request) {
	if !s.IsPeer(r) {
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}
	if !Config().EnableMigrate {
		w.Write([]byte("please set enable_migrate=true"))
		return
	}
	result := JsonResult{
		Status:  "ok",
		Message: "repair job start,don't try again,very danger!",
	}
	go s.RepairFileInfoFromFile()
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) Reload(w http.ResponseWriter, r *http.Request) {
	var (
		err             error
		data            []byte
		cfg             GlobalConfig
		action, cfgjson string
		result          JsonResult
	)
	result.Status = "fail"
	r.ParseForm()
	if !s.IsPeer(r) {
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
		return
	}
	cfgjson = r.FormValue("cfg")
	action = r.FormValue("action")
	if action == "get" {
		result = JsonResult{
			Data:   Config(),
			Status: "ok",
		}
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	if action == "set" {
		if cfgjson == "" {
			result.Message = "(error) param cfg(json) require!"
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		if err = json.Unmarshal([]byte(cfgjson), &cfg); err != nil {
			logrus.Errorf("unmarshal error!cfgjson=%s;err=%+v", cfgjson, err)
			result.Message = err.Error()
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		result.Status = "ok"
		cfgjson = s.util.JsonEncodePretty(cfg)
		s.util.WriteFile(CONST_CONF_FILE_NAME, cfgjson)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	if action == "reload" {
		if data, err = ioutil.ReadFile(CONST_CONF_FILE_NAME); err != nil {
			result.Message = err.Error()
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		if err = json.Unmarshal(data, &cfg); err != nil {
			result.Message = err.Error()
			w.Write([]byte(s.util.JsonEncodePretty(result)))
			return
		}
		ParseConfig(CONST_CONF_FILE_NAME)
		//todo
		s.initComponent(true)
		result.Status = "ok"
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	if action == "" {
		w.Write([]byte("(error) action support set(json) get reload"))
	}
}

func (s *Server) RemoveEmptyDir(w http.ResponseWriter, r *http.Request) {
	result := JsonResult{
		Status: "ok",
	}

	if s.IsPeer(r) {
		go s.util.RemoveEmptyDir(DATA_DIR)
		go s.util.RemoveEmptyDir(STORE_DIR)
		result.Message = "clean job start ..,don't try again"
		w.Write([]byte(s.util.JsonEncodePretty(result)))
	} else {
		result.Message = s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
	}

}

func (s *Server) Backup(w http.ResponseWriter, r *http.Request) {
	var (
		err              error
		date, inner, url string
		result           JsonResult
	)
	result.Status = "ok"
	r.ParseForm()
	date = r.FormValue("date")
	inner = r.FormValue("inner")
	if date == "" {
		date = s.util.GetToDay()
	}
	if s.IsPeer(r) {
		if inner != "1" {
			for _, peer := range Config().Peers {
				go func() {
					url = fmt.Sprintf("%s%s", peer, s.getRequestURI("backup"))
					req := httplib.Post(url)
					req.Param("date", date)
					req.Param("inner", inner)
					req.SetTimeout(time.Second*5, time.Second*120)
					if _, err = req.String(); err != nil {
						logrus.Errorf("backup req error!err=+v", err)
					}
				}()
			}
		}
		go s.BackUpMetaDataByDate(date)
		result.Message = "backup job start ... "
		w.Write([]byte(s.util.JsonEncodePretty(result)))
	} else {
		result.Message = s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
	}
}

// Notice: performance is poor,just for low capacity,but low memory , if you want to high performance,use searchMap for search,but memory ....
func (s *Server) Search(w http.ResponseWriter, r *http.Request) {
	var (
		result    JsonResult
		err       error
		kw        string
		count     int
		fileInfos []FileInfo
		md5s      []string
	)
	kw = r.FormValue("kw")
	if !s.IsPeer(r) {
		result.Message = s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	it := s.ldb.NewIterator(nil, nil)
	defer it.Release()
	for it.Next() {
		var fileInfo FileInfo
		value := it.Value()
		if err = json.Unmarshal(value, &fileInfo); err != nil {
			logrus.Errorf("Unmarshal error!value=%+v;err=%+v", value, err)
			continue
		}
		if strings.Contains(fileInfo.Name, kw) && !s.util.Contains(fileInfo.Md5, md5s) {
			count++
			fileInfos = append(fileInfos, fileInfo)
			md5s = append(md5s, fileInfo.Md5)
		}
		if count >= 100 {
			break
		}
	}
	result = JsonResult{
		Status: "ok",
		Data:   fileInfos,
	}
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) SearchDict(kw string) []FileInfo {
	var fileInfos []FileInfo

	for tuple := range s.searchMap.Iter() {
		if strings.Contains(tuple.Val.(string), kw) {
			if fileInfo, err := s.GetFileInfoFromLevelDB(tuple.Key); fileInfo != nil && err == nil {
				fileInfos = append(fileInfos, *fileInfo)
			}
		}
	}
	return fileInfos
}

func (s *Server) ListDir(w http.ResponseWriter, r *http.Request) {
	var (
		result      JsonResult
		dir, tmpDir string
		fileInfos   []os.FileInfo
		err         error
		fileResult  []FileInfoResult
	)
	if !s.IsPeer(r) {
		result.Message = s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	dir = r.FormValue("dir")
	dir = strings.Replace(dir, ".", "", -1)
	if tmpDir, err = os.Readlink(dir); tmpDir != "" && err == nil {
		dir = tmpDir
	}
	fileInfos, err = ioutil.ReadDir(DOCKER_DIR + STORE_DIR_NAME + "/" + dir)
	if err != nil {
		logrus.Errorf("error=%+v", err)
		result.Message = err.Error()
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}

	for _, f := range fileInfos {
		fi := FileInfoResult{
			Name:    f.Name(),
			Size:    f.Size(),
			IsDir:   f.IsDir(),
			ModTime: f.ModTime().Unix(),
			Path:    dir,
			Md5:     s.util.MD5(strings.Replace(STORE_DIR_NAME+"/"+dir+"/"+f.Name(), "//", "/", -1)),
		}
		fileResult = append(fileResult, fi)
	}
	result = JsonResult{
		Status: "ok",
		Data:   fileResult,
	}
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

/**
discrepancy:差异; 不符合; 不一致;

*/
func (s *Server) VerifyGoogleCode(secret string, code string, discrepancy int64) bool {
	goauth := googleAuthenticator.NewGAuth()
	if ok, err := goauth.VerifyCode(secret, code, discrepancy); ok {
		return ok
	} else {
		logrus.Errorf("error=%+v", err)
		return ok
	}
}

func (s *Server) GenGoogleCode(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		result JsonResult
		secret string
		goauth *googleAuthenticator.GAuth
	)
	r.ParseForm()
	goauth = googleAuthenticator.NewGAuth()
	secret = r.FormValue("secret")
	result = JsonResult{
		Status:  "ok",
		Message: "ok",
	}
	if !s.IsPeer(r) {
		result.Message = s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	if result.Data, err = goauth.GetCode(secret); err != nil {
		result.Message = err.Error()
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) GenGoogleSecret(w http.ResponseWriter, r *http.Request) {
	result := JsonResult{
		Status:  "ok",
		Message: "ok",
	}
	if !s.IsPeer(r) {
		result.Message = s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
		return
	}
	//getseed
	seeds := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	str := ""
	random.Seed(time.Now().Unix())
	for i := 0; i < 16; i++ {
		str += string(seeds[random.Intn(32)])
	}
	result.Data = s
	w.Write([]byte(s.util.JsonEncodePretty(result)))
}

func (s *Server) Report(w http.ResponseWriter, r *http.Request) {
	var (
		reportFileName string
		result         JsonResult
		html           string
	)
	result.Status = "ok"
	r.ParseForm()
	if s.IsPeer(r) {
		reportFileName = STATIC_DIR + "/report.html"
		if s.util.IsExist(reportFileName) {
			if data, err := s.util.ReadBinFile(reportFileName); err != nil {
				logrus.Errorf("ReadBinFile error!err=%+v", err)
				result.Message = err.Error()
				w.Write([]byte(s.util.JsonEncodePretty(result)))
				return
			} else {
				html = string(data)
				if Config().SupportGroupManage {
					html = strings.Replace(html, "{group}", "/"+Config().Group, 1)
				} else {
					html = strings.Replace(html, "{group}", "", 1)
				}
				w.Write([]byte(html))
				return
			}
		} else {
			w.Write([]byte(fmt.Sprintf("%s is not found!", reportFileName)))
		}
	} else {
		w.Write([]byte(s.GetClusterNotPermitMessage(r)))
	}
}

func (s *Server) Repair(w http.ResponseWriter, r *http.Request) {
	var (
		force       string
		forceRepair bool
		result      JsonResult
	)
	result.Status = "ok"
	r.ParseForm()
	force = r.FormValue("force")
	if force == "1" {
		forceRepair = true
	}
	if s.IsPeer(r) {
		go s.AutoRepair(forceRepair)
		result.Message = "repair job start ... "
		w.Write([]byte(s.util.JsonEncodePretty(result)))
	} else {
		result.Message = s.GetClusterNotPermitMessage(r)
		w.Write([]byte(s.util.JsonEncodePretty(result)))
	}
}

func (s *Server) Status(w http.ResponseWriter, r *http.Request) {
	var (
		status        JsonResult
		sts           map[string]interface{}
		today, appDir string
		sumset        mapset.Set
		ok            bool
		v             interface{}
		err           error
		diskInfo      *disk.UsageStat
		memInfo       *mem.VirtualMemoryStat
	)
	memStat := new(runtime.MemStats)
	runtime.ReadMemStats(memStat)
	today = s.util.GetToDay()
	sts = make(map[string]interface{})
	sts["Fs.QueueFromPeers"] = len(s.queueFromPeers)
	sts["Fs.QueueToPeers"] = len(s.queueToPeers)
	sts["Fs.QueueFileLog"] = len(s.queueFileLog)

	for _, k := range []string{CONST_FILE_Md5_FILE_NAME, CONST_Md5_ERROR_FILE_NAME, CONST_Md5_QUEUE_FILE_NAME} {
		k2 := fmt.Sprintf("%s_%s", today, k)
		if v, ok = s.sumMap.GetValue(k2); ok {
			sumset = v.(mapset.Set)
			if k == CONST_Md5_QUEUE_FILE_NAME {
				sts["Fs.QueueSetSize"] = sumset.Cardinality()
			} else if k == CONST_Md5_ERROR_FILE_NAME {
				sts["Fs.ErrorSetSize"] = sumset.Cardinality()
			} else if k == CONST_FILE_Md5_FILE_NAME {
				sts["Fs.FileSetSize"] = sumset.Cardinality()
			}
		}
	}

	sts["Fs.AutoRepair"] = Config().AutoRepair
	sts["Fs.QueueUpload"] = len(s.queueUpload)
	sts["Fs.RefreshInterval"] = Config().RefreshInterval
	sts["Fs.Peers"] = Config().Peers
	sts["Fs.Local"] = s.host
	sts["Fs.FileStats"] = s.GetStat()
	sts["Fs.ShowDir"] = Config().ShowDir
	sts["Sys.NumGoroutne"] = runtime.NumGoroutine()
	sts["Sys.NumCpu"] = runtime.NumCPU()
	sts["Sys.Alloc"] = memStat.Alloc
	sts["Sys.TotalAlloc"] = memStat.TotalAlloc
	sts["Sys.HeapAlloc"] = memStat.HeapAlloc
	sts["Sys.Frees"] = memStat.Frees
	sts["Sys.NumGC"] = memStat.NumGC
	sts["Sys.GCCPUFraction"] = memStat.GCCPUFraction
	sts["Sys.GCSys"] = memStat.GCSys
	//sts["Sys.MemInfo"]=memStat
	if appDir, err = filepath.Abs("."); err != nil {
		logrus.Errorf("abs error!err=%+v", err)
	}
	if diskInfo, err = disk.Usage(appDir); err != nil {
		logrus.Errorf("disk usage error!err=%+v", err)
	}
	sts["Sys.DiskInfo"] = diskInfo

	if memInfo, err = mem.VirtualMemory(); err != nil {
		logrus.Errorf("virtualMemory error!err=%+v", err)
	}
	sts["Sys.MemInfo"] = memInfo
	status.Status = "ok"
	status.Data = sts
	w.Write([]byte(s.util.JsonEncodePretty(status)))
}

func (s *Server) HeartBeat(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	var uploadUrl, uploadBigUrl, uppy string

	uploadUrl = "/upload"
	uploadBigUrl = CONST_BIG_UPLOAD_PATH_SUFFIX
	if Config().EnableWebUpload {
		uploadUrl = fmt.Sprintf("/%s/upload", Config().Group)
		uploadBigUrl = fmt.Sprintf("/%s%s", Config().Group, CONST_BIG_UPLOAD_PATH_SUFFIX)
		uppy = `<html>
			  
			  <head>
				<meta charset="utf-8" />
				<title>go-fastdfs</title>
				<style>form { bargin } .form-line { display:block;height: 30px;margin:8px; } #stdUpload {background: #fafafa;border-radius: 10px;width: 745px; }</style>
				<link href="https://transloadit.edgly.net/releases/uppy/v0.30.0/dist/uppy.min.css" rel="stylesheet"></head>
			  
			  <body>
                <div>标准上传(强列建议使用这种方式)</div>
				<div id="stdUpload">
				  
				  <form action="%s" method="post" enctype="multipart/form-data">
					<span class="form-line">文件(file):
					  <input type="file" id="file" name="file" /></span>
					<span class="form-line">场景(scene):
					  <input type="text" id="scene" name="scene" value="%s" /></span>
					<span class="form-line">文件名(filename):
					  <input type="text" id="filename" name="filename" value="" /></span>
					<span class="form-line">输出(output):
					  <input type="text" id="output" name="output" value="json2" title="json|text|json2" /></span>
					<span class="form-line">自定义路径(path):
					  <input type="text" id="path" name="path" value="" /></span>
	              <span class="form-line">google认证码(code):
					  <input type="text" id="code" name="code" value="" /></span>
					 <span class="form-line">自定义认证(auth_token):
					  <input type="text" id="auth_token" name="auth_token" value="" /></span>
					<input type="submit" name="submit" value="upload" />
                </form>
				</div>
                 <div>断点续传（如果文件很大时可以考虑）</div>
				<div>
				 
				  <div id="drag-drop-area"></div>
				  <script src="https://transloadit.edgly.net/releases/uppy/v0.30.0/dist/uppy.min.js"></script>
				  <script>var uppy = Uppy.Core().use(Uppy.Dashboard, {
					  inline: true,
					  target: '#drag-drop-area'
					}).use(Uppy.Tus, {
					  endpoint: '%s'
					})
					uppy.on('complete', (result) => {
					 // console.log(result) console.log('Upload complete! We’ve uploaded these files:', result.successful)
					})
					//uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca',callback_url:'http://127.0.0.1/callback' ,filename:'自定义文件名','path':'自定义path',scene:'自定义场景' })//这里是传递上传的认证参数,callback_url参数中 id为文件的ID,info 文转的基本信息json
					uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca',callback_url:'http://127.0.0.1/callback'})//自定义参数与普通上传类似（虽然支持自定义，建议不要自定义，海量文件情况下，自定义很可能给自已给埋坑）
                </script>
				</div>
			  </body>
			</html>`
		uppyFileName := STATIC_DIR + "/uppy.html"
		if s.util.IsExist(uppyFileName) {
			if data, err := s.util.ReadBinFile(uppyFileName); err != nil {
				logrus.Errorf("ReadBinFile error!uppyFileName=%s;err=%+v;", uppyFileName, err)
			} else {
				uppy = string(data)
			}
		} else {
			s.util.WriteFile(uppyFileName, uppy)
		}

	} else {
		w.Write([]byte("web upload deny!"))
	}
}

func init() {
	flag.Parse()
	if *v {
		fmt.Sprintf("%s\n%s\n%s\n%s\n", VERSION, BUILD_TIME, GO_VERSION, GIT_VERSION)
		os.Exit(0)
	}
	appDir, e1 := filepath.Abs(filepath.Dir(os.Args[0]))
	curDir, e2 := filepath.Abs(".")
	if e1 == nil && e2 == nil && appDir != curDir && !strings.Contains(appDir, "go-build") {
		msg := fmt.Sprintf("please change directory to '%s' start fielserver\n", appDir)
		msg += fmt.Sprintf("请切换到 '%s' 目录启动 fileserver ", appDir)
		logrus.Warnf(msg)
		os.Exit(1)
	}
	DOCKER_DIR = os.Getenv("GO_FASTDFS_DIR")
	if DOCKER_DIR != "" && !strings.HasPrefix(DOCKER_DIR, "/") {
		DOCKER_DIR += "/"
	}
	STORE_DIR = DOCKER_DIR + STORE_DIR_NAME
	CONF_DIR = DOCKER_DIR + CONF_DIR_NAME
	DATA_DIR = DOCKER_DIR + DATA_DIR_NAME
	LOG_DIR = DOCKER_DIR + LOG_DIR_NAME
	STATIC_DIR = DOCKER_DIR + STORE_DIR_NAME
	LARGE_DIR_NAME = "haystack"
	LARGE_DIR = STORE_DIR + "/haystack"
	CONST_LEVELDB_FILE_NAME = DATA_DIR + "/fileserver.db"
	CONST_LOG_LEVELDB_FILE_NAME = DATA_DIR + "/log.db"
	CONST_STAT_FILE_NAME = DATA_DIR + "/stat.json"
	CONST_CONF_FILE_NAME = CONF_DIR + "/cfg.json"
	CONST_SERVER_CRT_FILE_NAME = CONF_DIR + "/server.crt"
	CONST_SERVER_KEY_FILE_NAME = CONF_DIR + "/server.key"
	CONST_SEARCH_FILE_NAME = DATA_DIR + "/search.txt"
	FOLDERS = []string{DATA_DIR, STORE_DIR, CONF_DIR, STATIC_DIR}
	logAccessConfigStr = strings.Replace(logAccessConfigStr, "{DOCKER_DIR}", DOCKER_DIR, -1)
	logConfigStr = strings.Replace(logConfigStr, "{DOCKER_DIR}", DOCKER_DIR, -1)

	for _, folder := range FOLDERS {
		os.MkdirAll(folder, 0775)
	}
	server = NewServer()

	peerId := os.Getenv("GO_FASTDFS_PEER_ID")
	if peerId == "" {
		peerId = fmt.Sprintf("%d", server.util.RandInt(0, 9))
	}
	if !server.util.FileExists(CONST_CONF_FILE_NAME) {
		ip := os.Getenv("GO_FASTDFS_IP")
		if ip == "" {
			ip = server.util.GetPulicIP()
		}
		peer := "http://" + ip + ";3600"
		peers := os.Getenv("GO_FASTDFS_PEERS")
		if peers == "" {
			peers = peer
		}
		cfg := fmt.Sprintf(cfgJson, peerId, peer, peers)
		server.util.WriteFile(CONST_CONF_FILE_NAME, cfg)
	}

	if logger, err := log.LoggerFromConfigAsBytes([]byte(logConfigStr)); err != nil {
		panic(err)
	} else {
		log.ReplaceLogger(logger)
	}

	if _logacc, err := log.LoggerFromConfigAsBytes([]byte(logAccessConfigStr)); err == nil {
		logacc = _logacc
		logrus.Infof("success init log access!")
	} else {
		logrus.Errorf(err.Error())
	}
	ParseConfig(CONST_CONF_FILE_NAME)
	if ips, _ := server.util.GetAllIpsV4(); len(ips) > 0 {
		_ip := server.util.Match("\\d+\\.\\d+\\.\\d+\\.\\d+", Config().Host)
		if len(_ip) > 0 && !server.util.Contains(_ip[0], ips) {
			msg := fmt.Sprintf("host config is error,must in local ips:%s", strings.Join(ips, ","))
			logrus.Warnf(msg)
		}
	}
	if Config().QueueSize == 0 {
		Config().QueueSize = CONST_QUEUE_SIZE
	}
	if Config().PeerId == "" {
		Config().PeerId = peerId
	}
	if Config().SupportGroupManage {
		staticHandler = http.StripPrefix("/"+Config().Group+"/", http.FileServer(http.Dir(STORE_DIR)))
	} else {
		staticHandler = http.StripPrefix("/", http.FileServer(http.Dir(STORE_DIR)))
	}
	//todo
	server.initComponent(false)

}

type hookDataStore struct {
	tusd.DataStore
}

type httpError struct {
	error
	statusCode int
}

func (err *httpError) StatusCode() int {
	return err.statusCode
}

func (err *httpError) Body() []byte {
	return []byte(err.Error())
}

func (store *hookDataStore) NewUpload(info *tusd.FileInfo) (id string, err error) {
	var jsonResult JsonResult
	if Config().AuthUrl != "" {
		if auth_token, ok := info.MetaData["auth_token"]; !ok {
			msg := "token auth fail,auth_token is not in http header Upload-Metadata," +
				"in uppy uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca' })"
			str := fmt.Sprintf("msg=%s;current header:%+v", msg, info.MetaData)
			logrus.Error(str)
			return "", httpError{
				error:      errors.New(str),
				statusCode: 401,
			}
		} else {
			req := httplib.Post(Config().AuthUrl)
			req.Param("auth_token", auth_token)
			req.SetTimeout(time.Second*5, time.Second*10)
			content, err := req.String()

			if err != nil {
				logrus.Error(err)
				return "", err
			}
			content = strings.TrimSpace(content)

			if strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}") {
				if err = json.Unmarshal([]byte(content), &jsonResult); err != nil {
					logrus.Error(err)
					return "", httpError{
						error:      errors.New(err.Error() + ";\n" + content),
						statusCode: 401,
					}
				}
				if jsonResult.Data != "ok" {
					return "", httpError{
						error:      errors.New(content),
						statusCode: 401,
					}
				}

			} else {
				if strings.TrimSpace(content) != "ok" {
					return "", httpError{
						error:      errors.New(content),
						statusCode: 401,
					}
				}
			}
		}
	} //if authUrl;
	return store.DataStore.NewUpload(*info)
}

func (s *Server) uploadCompleteCallBack(info *tusd.FileInfo, fileInfo *FileInfo) {
	if callback_url, ok := info.MetaData["callback_url"]; ok {
		req := httplib.Post(callback_url)
		req.SetTimeout(time.Second*10, time.Second*10)
		req.Param("info", server.util.JsonEncodePretty(fileInfo))
		req.Param("id", info.ID)
		if _, err := req.String(); err != nil {
			logrus.Error(err)
		}
	}
}

var BIG_DIR string

func (s *Server) notify(handler *tusd.Handler) {

	for {
		select {
		case info := <-handler.CompleteUploads:
			logrus.Infof("CompleteUploads!info=%+v", info)
			name := ""
			pathCustom := ""
			scene := Config().DefaultScene
			if v, ok := info.MetaData["filename"]; ok {
				name = v
			}
			if v, ok := info.MetaData["scene"]; ok {
				scene = v
			}
			if v, ok := info.MetaData["path"]; ok {
				pathCustom = v
			}
			oldFullPath := BIG_DIR + "/" + info.ID + ".bin"
			infoFullPath := BIG_DIR + "/" + info.ID + ".info"

			md5sum := ""
			var err error
			if md5sum, err = s.util.GetFileSumByName(oldFullPath, Config().FileSumArithmetic); err != nil {
				logrus.Error(err)
				continue
			}
			ext := path.Ext(name)
			fileName := md5sum + ext
			if name != "" {
				fileName = name
			}
			if Config().RenameFile {
				fileName = md5sum + ext
			}
			timeStamp := time.Now().Unix()
			fpath := time.Now().Format("/20060102/15/04")
			if pathCustom != "" {
				fpath = "/" + strings.Replace(pathCustom, ".", "", -1) + "/"
			}
			newFullPath := STORE_DIR + "/" + scene + fpath + Config().PeerId + "/" + fileName
			if pathCustom != "" {
				newFullPath = STORE_DIR + "/" + scene + fpath + fileName
			}
			if fi, err := s.GetFileInfoFromLevelDB(md5sum); err != nil {
				logrus.Error(err)
			} else {
				tpath := s.GetFilePathByInfo(fi, true)
				var fileInfo *FileInfo
				if fi.Md5 != "" && s.util.FileExists(tpath) {
					if fileInfo, err = s.SaveFileInfoToLevelDB(info.ID, fi, s.ldb); err != nil {
						logrus.Error(err)
					}
					str := fmt.Sprintf("file is found md5:%s;remove file:%s,%s;", fi.Md5, oldFullPath, infoFullPath)
					logrus.Infof(str)
					os.Remove(oldFullPath)
					os.Remove(infoFullPath)
					go s.uploadCompleteCallBack(&info, fileInfo)
					continue
				}
			}
			fpath2 := STORE_DIR_NAME + "/" + Config().DefaultScene + fpath + Config().PeerId
			if pathCustom != "" {
				fpath2 = STORE_DIR_NAME + "/" + Config().DefaultScene + fpath
				fpath2 = strings.TrimRight(fpath2, "/")
			}

			os.MkdirAll(DOCKER_DIR+fpath2, 0775)
			fileInfo := &FileInfo{
				Name:      name,
				Path:      fpath2,
				ReName:    fileName,
				Size:      info.Size,
				TimeStamp: timeStamp,
				Md5:       md5sum,
				Peers:     []string{s.host},
				OffSet:    -1,
			}
			os.Remove(infoFullPath)
			if err := os.Rename(oldFullPath, newFullPath); err != nil {
				logrus.Error(err)
				continue
			}
			s.SaveFileMd5Log(fileInfo, CONST_FILE_Md5_FILE_NAME)
			go s.postFileToPeer(fileInfo)
			go s.uploadCompleteCallBack(&info, fileInfo)
			break
		} //select;
	} //for;
}

func (s *Server) initTus() {
	var (
		err     error
		fileLog *os.File
		bigDir  string
	)
	BIG_DIR = STORE_DIR + "/_big/" + Config().PeerId

	os.MkdirAll(BIG_DIR, 0775)
	os.MkdirAll(LOG_DIR, 0775)
	store := filestore.FileStore{
		Path: BIG_DIR,
	}
	if fileLog, err = os.OpenFile(LOG_DIR+"/tusd.log", os.O_CREATE|os.O_RDWR, 0666); err != nil {
		logrus.Error(err)
		panic("initTus error!")
	}
	go func() {
		//转移日志文件;
		for {
			if fi, err := fileLog.Stat(); err != nil {
				logrus.Error(err)
			} else {
				if fi.Size() > (500 << 20) {
					//500M
					s.util.CopyFile(LOG_DIR+"/tusd.log", LOG_DIR+"/tusd.log.2")
					fileLog.Seek(0, 0)
					fileLog.Truncate(0)
					fileLog.Seek(0, 2)
				}
			}
			time.Sleep(time.Second * 30)
		} //for
	}()

	l := slog.New(fileLog, "[tusd] ", slog.LstdFlags)
	bigDir = CONST_BIG_UPLOAD_PATH_SUFFIX

	if Config().SupportGroupManage {
		bigDir = fmt.Sprintf("/%s%s", Config().Group, CONST_BIG_UPLOAD_PATH_SUFFIX)
	}
	composer := tusd.NewStoreComposer()
	//support raw tus upload and download;
	store.GetReaderExt = func(id string) (io.Reader, error) {
		var (
			offset int64
			err    error
			length int
			buffer []byte
			fi     *FileInfo
			fn     string
		)
		if fi, err = s.GetFileInfoFromLevelDB(id); err != nil {
			logrus.Error(err)
			return nil, err
		} else {
			if Config().AuthUrl != "" {
				fileResult := s.util.JsonEncodePretty(s.BuildFileResult(fi, nil))
				bufferReader := bytes.NewBuffer([]byte(fileResult))
				return bufferReader, nil
			}
			fn = fi.Name
			if fi.ReName != "" {
				fn = fi.ReName
			}
			fp := DOCKER_DIR + fi.Path + "/" + fn

			if s.util.FileExists(fp) {
				logrus.Infof("download:%s", fp)
				return os.Open(fp)
			}
			if ps := strings.Split(fp, ","); len(ps) > 2 && s.util.FileExists(ps[0]) {
				if length, err = strconv.Atoi(ps[2]); err != nil {
					return nil, err
				}

				if offset, err = strconv.ParseInt(ps[1], 10, 64); err != nil {
					return nil, err
				}

				if buffer, err = s.util.ReadFileByOffSet(ps[0], offset, length); err != nil {
					return nil, err
				}
				if buffer[0] == '1' {
					bufferReader := bytes.NewBuffer(buffer[1:])
					return bufferReader, nil
				} else {
					msg := "data no sync"
					logrus.Error(msg)
					return nil, errors.New(msg)
				}
			}
			return nil, errors.New(fmt.Sprintf("%s not found!", fp))
		}
	}
	store.UseIn(composer)

	SetupPreHooks := func(composer *tusd.StoreComposer) {
		composer.UseCore(composer.Core)
	}
	SetupPreHooks(composer)
	handler, err := tusd.NewHandler(tusd.Config{
		Logger:                  l,
		BasePath:                bigDir,
		StoreComposer:           composer,
		NotifyCompleteUploads:   true,
		RespectForwardedHeaders: true,
	})
	go s.notify(handler)
	http.Handle(bigDir, http.StripPrefix(bigDir, handler))
}

func (s *Server) FormatStatInfo() {
	var (
		data  []byte
		err   error
		count int64
		stat  map[string]interface{}
	)
	if s.util.FileExists(CONST_STAT_FILE_NAME) {
		if data, err = s.util.ReadBinFile(CONST_STAT_FILE_NAME); err != nil {
			logrus.Error(err)
		} else {
			if err = json.Unmarshal(data, &stat); err != nil {
				logrus.Error(err)
			} else {
				for k, v := range stat {
					switch v.(type) {
					case float64:
						vv := strings.Split(fmt.Sprintf("%f", v), ".")[0]
						if count, err = strconv.ParseInt(vv, 10, 64); err != nil {
							logrus.Error(err)
						} else {
							s.statMap.Put(k, count)
						}
						break
					default:
						s.statMap.Put(k, v)
					}
				}
			}
		}

	} else {
		s.RepairStatByDate(s.util.GetToDay())
	}
}

func (s *Server) initComponent(isReload bool) {
	ip := ""
	if ip = os.Getenv("GO_FASTDFS_IP"); ip == "" {
		ip = s.util.GetPulicIP()
	}
	if Config().Host == "" {
		if len(strings.Split(Config().Addr, ":")) == 2 {
			server.host = fmt.Sprintf("http://%s:%s", ip, strings.Split(Config().Addr, ":")[1])
			Config().Host = server.host
		}
	} else {
		if strings.HasPrefix(Config().Addr, "http") {
			server.host = Config().Host
		} else {
			server.host = "http://" + Config().Host
		}
	}

	ex, _ := regexp.Compile("\\d+\\.\\d+\\.\\d+\\.\\d+")
	var peers []string
	for _, peer := range Config().Peers {
		if s.util.Contains(ip, ex.FindAllString(peer, -1)) || s.util.Contains("127.0.0.1", ex.FindAllString(peer, -1)) {
			continue
		}
		if strings.HasPrefix(peer, "http") {
			peers = append(peers, peer)
		} else {
			peers = append(peers, "http://"+peer)
		}
	}
	Config().Peers = peers
	if !isReload {
		s.FormatStatInfo()
		if Config().EnableTus {
			s.initTus()
		}
	}
	for _, s := range Config().Scenes {
		kv := strings.Split(s, ":")
		if len(kv) == 2 {
			s.sceneMap.Put(kv[0], kv[1])
		}
	}
	if Config().ReadTimeout == 0 {
		Config().ReadTimeout = 60 * 10
	}
	if Config().WriteTimeout == 0 {
		Config().WriteTimeout = 60 * 10
	}
	if Config().UploadWorker == 0 {
		Config().UploadWorker = runtime.NumCPU() + 4
		if runtime.NumCPU() < 4 {
			Config().UploadWorker = 8
		}
	}
	if Config().UploadQueueSize == 0 {
		Config().UploadQueueSize = 200
	}
	if Config().RetryCount == 0 {
		Config().RetryCount = 3
	}
	if Config().SyncDelay == 0 {
		Config().SyncDelay = 60
	}
	if Config().WatchChanSize == 0 {
		Config().WatchChanSize = 100000
	}
}

type HttpHandler struct {
}

func (HttpHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	status_code := "200"
	defer func(t time.Time) {
		logrus.Infof("[Access] %s | %s | %s | %s | %s | %s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			time.Since(t).String(),
			server.util.GetClientIp(req),
			req.Method, status_code, req.RequestURI)
	}(time.Now())
	defer func() {
		if err := recover(); err != nil {
			status_code = "500"
			resp.WriteHeader(500)
			print(err)
			buff := debug.Stack()
			log.Error(err)
			log.Error(string(buff))
		}
	}()

	if Config().EnableCrossOrigin {
		server.CrossOrigin(resp, req)
	}
	http.DefaultServeMux.ServeHTTP(resp, req)
}

func (s *Server) Start() {
	go func() {
		s.CheckFileAndSendToPeer(s.util.GetToDay(), CONST_Md5_ERROR_FILE_NAME, false)
		time.Sleep(time.Second * time.Duration(Config().RefreshInterval))
	}()

	go s.CleanAndBackup()
	go s.CheckClusterStatus()
	go s.LoadQueueSendToPeer()
	go s.ConsumerPostToPeer()
	go s.ConsumerLog()
	go s.ConsumerDownload()
	go s.ConsumerUpload()
	go s.RemoveDownloading()

	if Config().EnableFsnotify {
		go s.WatchFilesChange()
	}

	if Config().EnableMigrate {
		go s.RepairFileInfoFromFile()
	}
	if Config().AutoRepair {
		go func() {
			for {
				time.Sleep(time.Minute * 3)
				s.AutoRepair(false)
				time.Sleep(time.Minute * 60)
			}
		}()
	}
	groupRoute := ""
	if Config().SupportGroupManage {
		groupRoute = "/" + Config().Group
	}
	go func() {
		for {
			time.Sleep(time.Minute * 1)
			debug.FreeOSMemory()
		}
	}()
	uploadPage := "upload.html"
	if groupRoute == "" {
		http.HandleFunc(fmt.Sprintf("%s", "/"), s.Download)
		http.HandleFunc(fmt.Sprintf("/%s", uploadPage), s.Index)
	} else {
		http.HandleFunc("/", s.Download)
		http.HandleFunc(groupRoute, s.Download)
		http.HandleFunc(fmt.Sprintf("%s/%s", groupRoute, uploadPage), s.Index)
	}
	http.HandleFunc(fmt.Sprintf("%s/check_files_exist", groupRoute), s.CheckFileExist)
	http.HandleFunc(fmt.Sprintf("%s/check_file_exist", groupRoute), s.CheckFileExist)
	http.HandleFunc(fmt.Sprintf("%s/upload", groupRoute), s.Upload)
	http.HandleFunc(fmt.Sprintf("%s/delete", groupRoute), s.RemoveFile)
	http.HandleFunc(fmt.Sprintf("%s/get_file_info", groupRoute), s.GetFileInfo)
	http.HandleFunc(fmt.Sprintf("%s/sync", groupRoute), s.Sync)
	http.HandleFunc(fmt.Sprintf("%s/stat", groupRoute), s.Stat)
	http.HandleFunc(fmt.Sprintf("%s/repair_stat", groupRoute), s.RepairStatWeb)
	http.HandleFunc(fmt.Sprintf("%s/status", groupRoute), s.Status)
	http.HandleFunc(fmt.Sprintf("%s/repair", groupRoute), s.Repair)
	http.HandleFunc(fmt.Sprintf("%s/report", groupRoute), s.Report)
	http.HandleFunc(fmt.Sprintf("%s/backup", groupRoute), s.Backup)
	http.HandleFunc(fmt.Sprintf("%s/search", groupRoute), s.Search)
	http.HandleFunc(fmt.Sprintf("%s/list_dir", groupRoute), s.ListDir)
	http.HandleFunc(fmt.Sprintf("%s/remove_empty_dir", groupRoute), s.RemoveEmptyDir)
	http.HandleFunc(fmt.Sprintf("%s/repair_fileinfo", groupRoute), s.RepairFileInfo)
	http.HandleFunc(fmt.Sprintf("%s/reload", groupRoute), s.Reload)
	http.HandleFunc(fmt.Sprintf("%s/syncfile_info", groupRoute), s.SyncFileInfo)
	http.HandleFunc(fmt.Sprintf("%s/get_md5s_by_date", groupRoute), s.GetMd5sForWeb)
	http.HandleFunc(fmt.Sprintf("%s/receive_md5s", groupRoute), s.ReceiveMd5s)
	http.HandleFunc(fmt.Sprintf("%s/gen_google_secret", groupRoute), s.GenGoogleSecret)
	http.HandleFunc(fmt.Sprintf("%s/gen_google_code", groupRoute), s.GenGoogleCode)
	//http.HandleFunc(fmt.Sprintf("%s/static/",groupRoute),http.StripPrefix(fmt.Sprintf("%s/static/",groupRoute),http.FileServer(http.Dir("./static"))))
	//http.HandleFunc(fmt.Sprintf("%s/static/",groupRoute),http.FileServer)
	http.HandleFunc("/"+Config().Group+"/", s.Download)

	logrus.Infof("Listen on %+v", Config().Addr)

	globalConfig := Config()
	logrus.Infof("globalConfig=%+v\n", GlobalConfig{})

	if Config().EnableHttps {
		err := http.ListenAndServeTLS(Config().Addr, CONST_SERVER_CRT_FILE_NAME, CONST_SERVER_KEY_FILE_NAME, new(HttpHandler))
		logrus.Errorf(err)
	} else {
		srv := &http.Server{
			Addr:              Config().Addr,
			Handler:           new(HttpHandler),
			ReadTimeout:       time.Duration(Config().ReadTimeout) * time.Second,
			ReadHeaderTimeout: time.Duration(Config().ReadHeaderTimeout) * time.Second,
			WriteTimeout:      time.Duration(Config().WriteTimeout) * time.Second,
			IdleTimeout:       time.Duration(Config().IdleTimeout) * time.Second,
		}
		logrus.Info("srv=%+v", srv)
		if err := srv.ListenAndServe(); err != nil {
			logrus.Errorf("srv.ListenAndServe!err=%+v", err)
		}
	}
}

func main() {
	logrus.SetReportCaller(true)
	server.Start()
}
