package config

import (
	"sync"

	"gopkg.in/ini.v1"
)

type AppSection struct {
	Secret     string `json:"secret"`
	UploadPath string `json:"uploadPath"`
	BaseUrl    string `json:"baseUrl"`
}

type HttpSection struct {
	Port string `json:"port"`
}

type Config struct {
	App  AppSection  `json:"app"`
	Http HttpSection `json:"http"`
}

var lock = &sync.Mutex{}
var config *Config

func GetConfig() *Config {
	if config == nil {
		lock.Lock()
		defer lock.Unlock()
		config = loadConfig()
	}
	return config
}

func loadConfig() *Config {
	config = &Config{}
	iniData, err := ini.Load("config.ini")
	if err != nil {
		return checkConfig(nil)
	}
	appSection := iniData.Section("app")
	httpSection := iniData.Section("http")
	config.App = AppSection{
		Secret:     appSection.Key("secret").String(),
		UploadPath: appSection.Key("uploadPath").String(),
		BaseUrl:    appSection.Key("baseUrl").String(),
	}
	config.Http = HttpSection{
		Port: httpSection.Key("port").String(),
	}
	checkConfig(iniData)
	return config
}

func checkConfig(iniData *ini.File) *Config {
	/* config.App = AppSection{
		Secret: "default-secret",
	}
	config.Http = HttpSection{
		Port: "8000",
	} */
	//write to file
	if iniData == nil {
		iniData = ini.Empty()
	}
	appSection, err := iniData.NewSection("app")
	if err != nil {
		panic(err)
	}
	if config.App.Secret == "" {
		config.App.Secret = "default-secret"
		_, err = appSection.NewKey("secret", "default-secret")
		if err != nil {
			panic(err)
		}
	}
	if config.App.UploadPath == "" {
		config.App.UploadPath = "uploads"
		_, err = appSection.NewKey("uploadPath", "uploads")
		if err != nil {
			panic(err)
		}
	}
	if config.App.BaseUrl == "" {
		config.App.BaseUrl = "https://cdn.haimomgta5.vn"
		_, err = appSection.NewKey("baseUrl", config.App.BaseUrl)
		if err != nil {
			panic(err)
		}
	}
	//HTTP
	httpSection, err := iniData.NewSection("http")
	if err != nil {
		panic(err)
	}
	if config.Http.Port == "" {
		config.Http.Port = "1107"
		_, err = httpSection.NewKey("port", "1107")
		if err != nil {
			panic(err)
		}
	}

	//SAVE
	err = iniData.SaveTo("config.ini")
	if err != nil {
		panic(err)
	}

	return config
}
