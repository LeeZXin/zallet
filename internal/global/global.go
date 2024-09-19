package global

import (
	"github.com/LeeZXin/zallet/internal/util"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
	"xorm.io/xorm"
)

var (
	InstanceId string
	BaseDir    string
	SockFile   string
	Viper      *viper.Viper
	Xengine    *xorm.Engine
	AppPath    string
	SshHost    string
	SshToken   string
)

func Init() {
	if InstanceId == "" {
		AppPath = util.GetAppPath()
		pwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("os.Getwd err: %v", err)
		}
		BaseDir = filepath.Join(pwd, "data")
		err = os.MkdirAll(BaseDir, os.ModePerm)
		if err != nil {
			log.Fatalf("os.MkdirAll: %s err: %v", BaseDir, err)
		}
		SockFile = filepath.Join(BaseDir, "zallet.sock")
		InstanceId = readInstanceId()
		Viper = newViper()
		Xengine = newXormEngine()
		SshHost = Viper.GetString("ssh.agent.host")
		if SshHost == "" {
			SshHost = "127.0.0.1:6666"
		}
		SshToken = Viper.GetString("ssh.agent.token")
	}
}

func readInstanceId() string {
	instanceFilePath := filepath.Join(BaseDir, "instance")
	file, err := os.ReadFile(instanceFilePath)
	if err == nil {
		instanceId := string(file)
		if regexp.MustCompile(`^\w{32}$`).MatchString(instanceId) {
			return instanceId
		}
	} else if !os.IsNotExist(err) {
		log.Fatalf("Read instance file failed with err: %v", err)
	}
	instanceId := util.RandomUuid()
	err = os.WriteFile(instanceFilePath, []byte(instanceId), os.ModePerm)
	if err != nil {
		log.Fatalf("write instance file failed with err:%v", err)
	}
	return instanceId
}

func newViper() *viper.Viper {
	// 最终配置
	v := viper.New()
	v.SetConfigType("yaml")
	v.AddConfigPath(BaseDir)
	v.SetConfigName("application.yaml")
	_ = v.ReadInConfig()
	return v
}

func newXormEngine() *xorm.Engine {
	x, err := xorm.NewEngine("mysql", Viper.GetString("xorm.dataSourceName"))
	if err != nil {
		log.Fatalf("init xorm failed with err:%v", err)
	}
	x.SetMaxIdleConns(1)
	x.SetConnMaxLifetime(time.Hour)
	return x
}
