package zallet

import (
	"github.com/LeeZXin/zallet/internal/static"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"time"
	"xorm.io/xorm"
)

func (s *Server) initXorm() {
	var err error
	driverName := static.GetString("xorm.driverName")
	if driverName == "" {
		driverName = "mysql"
	}
	s.xengine, err = xorm.NewEngine(driverName, static.GetString("xorm.dataSourceName"))
	if err != nil {
		log.Fatalf("init xorm failed with err:%v", err)
	}
	s.xengine.SetMaxIdleConns(1)
	s.xengine.SetConnMaxLifetime(time.Hour)
}
