/*
 * Copyright (c) 2022, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package conf

import (
	"io/ioutil"
	httpClient "net/http"
	netUrl "net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/megaease/easeprobe/channel"
	"github.com/megaease/easeprobe/global"
	"github.com/megaease/easeprobe/notify"
	"github.com/megaease/easeprobe/probe"
	"github.com/megaease/easeprobe/probe/client"
	"github.com/megaease/easeprobe/probe/host"
	"github.com/megaease/easeprobe/probe/http"
	"github.com/megaease/easeprobe/probe/shell"
	"github.com/megaease/easeprobe/probe/ssh"
	"github.com/megaease/easeprobe/probe/tcp"
	"github.com/megaease/easeprobe/probe/tls"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var config *Conf

// Get return the global configuration
func Get() *Conf {
	return config
}

// Schedule is the schedule.
type Schedule int

//
const (
	Hourly Schedule = iota
	Daily
	Weekly
	Monthly
	None
)

// UnmarshalYAML is unmarshal the debug level
func (s *Schedule) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var level string
	if err := unmarshal(&level); err != nil {
		return err
	}
	switch strings.ToLower(level) {
	case "hourly":
		*s = Hourly
	case "daily":
		*s = Daily
	case "weekly":
		*s = Weekly
	case "monthly":
		*s = Monthly
	default:
		*s = None
	}
	return nil
}

// Notify is the settings of notification
type Notify struct {
	Retry global.Retry `yaml:"retry"`
	Dry   bool         `yaml:"dry"`
}

// Probe is the settings of prober
type Probe struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

// SLAReport is the settings for SLA report
type SLAReport struct {
	Schedule Schedule `yaml:"schedule"`
	Time     string   `yaml:"time"`
	Debug    bool     `yaml:"debug"`
	DataFile string   `yaml:"data"`
	Backups  int      `yaml:"backups"`
	Channels []string `yaml:"channels"`
}

// HTTPServer is the settings of http server
type HTTPServer struct {
	IP              string        `yaml:"ip"`
	Port            string        `yaml:"port"`
	AutoRefreshTime time.Duration `yaml:"refresh"`
	AccessLog       Log           `yaml:"log"`
}

// Settings is the EaseProbe configuration
type Settings struct {
	Name       string     `yaml:"name"`
	IconURL    string     `yaml:"icon"`
	PIDFile    string     `yaml:"pid"`
	Log        Log        `yaml:"log"`
	TimeFormat string     `yaml:"timeformat"`
	Probe      Probe      `yaml:"probe"`
	Notify     Notify     `yaml:"notify"`
	SLAReport  SLAReport  `yaml:"sla"`
	HTTPServer HTTPServer `yaml:"http"`
}

// Conf is Probe configuration
type Conf struct {
	Version  string          `yaml:"version"`
	HTTP     []http.HTTP     `yaml:"http"`
	TCP      []tcp.TCP       `yaml:"tcp"`
	Shell    []shell.Shell   `yaml:"shell"`
	Client   []client.Client `yaml:"client"`
	SSH      ssh.SSH         `yaml:"ssh"`
	TLS      []tls.TLS       `yaml:"tls"`
	Host     host.Host       `yaml:"host"`
	Notify   notify.Config   `yaml:"notify"`
	Settings Settings        `yaml:"settings"`
}

// Check if string is a url
func isExternalURL(url string) bool {
	if _, err := netUrl.ParseRequestURI(url); err != nil {
		log.Debugf("ParseRequestedURI: %s failed to parse with error %v", url, err)
		return false
	}

	parts, err := netUrl.Parse(url)
	if err != nil || parts.Host == "" || !strings.HasPrefix(parts.Scheme, "http") {
		log.Debugf("Parse: %s failed Scheme: %s, Host: %s (err: %v)", url, parts.Scheme, parts.Host, err)
		return false
	}

	return true
}

func getYamlFileFromInternet(url string) ([]byte, error) {
	r, err := httpClient.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if os.Getenv("HTTP_AUTHORIZATION") != "" {
		r.Header.Set("Authorization", os.Getenv("HTTP_AUTHORIZATION"))
	}

	httpClientObject := httpClient.Client{}
	if os.Getenv("HTTP_TIMEOUT") != "" {
		timeout, err := strconv.ParseInt(os.Getenv("HTTP_TIMEOUT"), 10, 64)
		if err != nil {
			return nil, err
		}
		httpClientObject.Timeout = time.Duration(timeout) * time.Second
	}

	resp, err := httpClientObject.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func getYamlFileFromFile(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	return ioutil.ReadFile(path)
}

func getYamlFile(path string) ([]byte, error) {
	if isExternalURL(path) {
		return getYamlFileFromInternet(path)
	}
	return getYamlFileFromFile(path)
}

// New read the configuration from yaml
func New(conf *string) (*Conf, error) {
	c := Conf{
		HTTP:   []http.HTTP{},
		TCP:    []tcp.TCP{},
		Shell:  []shell.Shell{},
		Client: []client.Client{},
		SSH: ssh.SSH{
			Bastion: &ssh.BastionMap,
			Servers: []ssh.Server{},
		},
		TLS: []tls.TLS{},
		Host: host.Host{
			Bastion: &host.BastionMap,
			Servers: []host.Server{},
		},
		Notify: notify.Config{},
		Settings: Settings{
			Name:       global.DefaultProg,
			IconURL:    global.DefaultIconURL,
			PIDFile:    filepath.Join(global.GetWorkDir(), global.DefaultPIDFile),
			Log:        NewLog(),
			TimeFormat: "2006-01-02 15:04:05 UTC",
			Probe: Probe{
				Interval: time.Second * 60,
				Timeout:  time.Second * 10,
			},
			Notify: Notify{
				Retry: global.Retry{
					Times:    3,
					Interval: time.Second * 5,
				},
				Dry: false,
			},
			SLAReport: SLAReport{
				Schedule: Daily,
				Time:     "00:00",
				Debug:    false,
				DataFile: global.DefaultDataFile,
				Backups:  global.DefaultMaxBackups,
				Channels: []string{global.DefaultChannelName},
			},
			HTTPServer: HTTPServer{
				IP:        global.DefaultHTTPServerIP,
				Port:      global.DefaultHTTPServerPort,
				AccessLog: NewLog(),
			},
		},
	}
	y, err := getYamlFile(*conf)
	if err != nil {
		log.Errorf("error: %v ", err)
		return &c, err
	}

	y = []byte(os.ExpandEnv(string(y)))

	err = yaml.Unmarshal(y, &c)
	if err != nil {
		log.Errorf("error: %v", err)
		return &c, err
	}

	// Initialization
	global.InitEaseProbe(c.Settings.Name, c.Settings.IconURL)
	c.initData()

	ssh.BastionMap.ParseAllBastionHost()
	host.BastionMap.ParseAllBastionHost()

	// pass the dry run to the channel
	channel.SetDryNotify(c.Settings.Notify.Dry)

	config = &c

	log.Infoln("Load the configuration file successfully!")
	if log.GetLevel() >= log.DebugLevel {
		s, err := yaml.Marshal(c)
		if err != nil {
			log.Debugf("%v\n%+v", err, c)
		} else {
			log.Debugf("\n%s", string(s))
		}
	}

	return &c, err
}

// InitAllLogs initialize all logs
func (conf *Conf) InitAllLogs() {

	conf.Settings.Log.InitLog(nil)
	conf.Settings.Log.LogInfo("Application")

	conf.Settings.HTTPServer.AccessLog.InitLog(log.New())
	conf.Settings.HTTPServer.AccessLog.LogInfo("Web Access")
}

func (conf *Conf) initData() {

	// Check if we are explicitly disabled
	if strings.TrimSpace(conf.Settings.SLAReport.DataFile) == "-" {
		log.Infof("SLA data disabled by configuration. Skipping SLA data store...")
		return
	}

	// Check if we are empty and use global.DefaultDataFile
	if strings.TrimSpace(conf.Settings.SLAReport.DataFile) == "" {
		conf.Settings.SLAReport.DataFile = global.DefaultDataFile
	}

	dir, _ := filepath.Split(conf.Settings.SLAReport.DataFile)
	// if dir part is not empty
	if strings.TrimSpace(dir) != "" {
		// check for `dir`` existence and create intermediate folders
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			log.Infof("Creating base directory for data file!")
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Warnf("Failed to create base directory for data file: %s", err.Error())
				return
			}
		}
	}

	// check if the data file exists and is a regular file
	dataInfo, err := os.Stat(conf.Settings.SLAReport.DataFile)
	if os.IsNotExist(err) || !dataInfo.Mode().IsRegular() {
		log.Infof("The data file %s, was not found!", conf.Settings.SLAReport.DataFile)
		return
	}

	if err := probe.LoadDataFromFile(conf.Settings.SLAReport.DataFile); err != nil {
		log.Warnf("Cannot load data from file(%s): %v", conf.Settings.SLAReport.DataFile, err)
	}

	probe.CleanDataFile(conf.Settings.SLAReport.DataFile, conf.Settings.SLAReport.Backups)
}

// isProbe checks whether a interface is a probe type
func isProbe(t reflect.Type) bool {
	modelType := reflect.TypeOf((*probe.Prober)(nil)).Elem()
	return t.Implements(modelType)
}

// AllProbers return all probers
func (conf *Conf) AllProbers() []probe.Prober {
	log.Debugf("--------- Process the probers settings ---------")
	return allProbersHelper(*conf)
}

func allProbersHelper(i interface{}) []probe.Prober {

	var probers []probe.Prober
	t := reflect.TypeOf(i)
	v := reflect.ValueOf(i)
	if t.Kind() != reflect.Struct {
		return probers
	}

	for i := 0; i < t.NumField(); i++ {
		tField := t.Field(i).Type.Kind()
		if tField == reflect.Struct {
			probers = append(probers, allProbersHelper(v.Field(i).Interface())...)
			continue
		}
		if tField != reflect.Slice {
			continue
		}

		vField := v.Field(i)
		for j := 0; j < vField.Len(); j++ {
			if !isProbe(vField.Index(j).Addr().Type()) {
				//log.Debugf("%s is not a probe type", vField.Index(j).Type())
				continue
			}

			log.Debugf("--> %s / %s / %+v", t.Field(i).Name, t.Field(i).Type.Kind(), vField.Index(j))
			probers = append(probers, vField.Index(j).Addr().Interface().(probe.Prober))
		}
	}

	return probers
}

// isNotify checks whether a interface is a Notify type
func isNotify(t reflect.Type) bool {
	modelType := reflect.TypeOf((*notify.Notify)(nil)).Elem()
	return t.Implements(modelType)
}

// AllNotifiers return all notifiers
func (conf *Conf) AllNotifiers() []notify.Notify {
	var notifies []notify.Notify

	log.Debugf("--------- Process the notification settings ---------")
	t := reflect.TypeOf(conf.Notify)
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Type.Kind() != reflect.Slice {
			continue
		}
		v := reflect.ValueOf(conf.Notify).Field(i)
		for j := 0; j < v.Len(); j++ {
			if !isNotify(v.Index(j).Addr().Type()) {
				log.Debugf("%s is not a notify type", v.Index(j).Type())
				continue
			}
			log.Debugf("--> %s - %s - %+v", t.Field(i).Name, t.Field(i).Type.Kind(), v.Index(j))
			notifies = append(notifies, v.Index(j).Addr().Interface().(notify.Notify))
		}
	}

	return notifies
}
