package wx

import (
	"flag"
	"github.com/mlogclub/simple"
	"time"

	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/sirupsen/logrus"
	"github.com/songtianyi/wechat-go/wxweb"
)

func WxFunc() {
 session,err:=wxweb.CreateSession(nil,nil,wxweb.TERMINAL_MODE)

 logrus.Infof("session=%+v;err=%+v;",session,err)



}
