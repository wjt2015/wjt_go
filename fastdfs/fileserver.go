package fastdfs
/**
参考:
https://gitee.com/linux2014/go-fastdfs_2
 */
import(
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type Server struct{
	ldb *leveldb.DB;
	logDB * leveldb.DB;



}




