package config

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path"
	"tiny_cni/internal/log"
	"tiny_cni/pkg/ipam"

	"github.com/alexflint/go-filemutex"
	"github.com/containernetworking/cni/pkg/types"
)

const (
	StorageDir         = "/run/tcni/"
	StorageFileName    = "config.json"
	StoragePath        = StorageDir + StorageFileName
	PlugNetworkCfgPath = "/etc/cni/net/net-conf.json"
)

type PlugStorage struct {
	Ipv4Record *ipam.Record         `json:"ipv4"`
	Mtx        *filemutex.FileMutex `json:"-"`
}
type Cfg struct {
	types.NetConf
}
type PlugNetworkCfg struct {
	Network string
	Backend map[string]string
}

func LoadCfg(data []byte) (*Cfg, error) {
	cfg := &Cfg{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
func newFileMutex(lockPath string) (*filemutex.FileMutex, error) {
	stat, err := os.Stat(lockPath)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		lockPath = path.Join(lockPath, "lock")
	}

	mtx, err := filemutex.New(lockPath)
	if err != nil {
		return nil, err
	}

	return mtx, nil
}
func LoadStorage() (*PlugStorage, error) {
	var err error
	if _, err = os.Stat(StoragePath); errors.Is(err, os.ErrNotExist) {
		if err = os.MkdirAll(StorageDir, 0750); err != nil {
			return nil, err
		}
		if file, err := os.Create(StoragePath); err != nil {
			log.Log.Fatal("Init config failed:", err)
		} else {
			if err = file.Close(); err != nil {
				log.Log.Fatal("Close config failed:", err)
			}
		}
		err = nil
	}
	if err != nil {
		log.Log.Debugf("%#v", err)
		return nil, err
	}
	mtx, err := newFileMutex(StoragePath)
	if err != nil {
		log.Log.Debug(err)
		return nil, err
	}
	storage := &PlugStorage{Mtx: mtx}
	storage.Lock()
	ok := storage.load()
	if !ok {
		storage.Unlock()
		log.Log.Fatal("load failed")
	}
	storage.Unlock()
	return storage, nil
}
func (s *PlugStorage) Lock() {
	err := s.Mtx.Lock()
	if err != nil {
		log.Log.Fatal("FileMutex Lock Failed:", err)
	}
}
func (s *PlugStorage) Unlock() {
	err := s.Mtx.Unlock()
	if err != nil {
		log.Log.Fatal("FileMutex Lock Failed:", err)
	}
}
func (s *PlugStorage) load() bool {
	data, err := os.ReadFile(StoragePath)
	if err != nil {
		s.Unlock()
		log.Log.Fatal("Read Config Failed", err)
	}
	if len(data) < 2 {
		log.Log.Warn("Empty Config: May be first run this plug in this node?")
		//s.Ipv4Record = s.
		cfg := &PlugNetworkCfg{}
		//json.Unmarshal()
		data, err := os.ReadFile(PlugNetworkCfgPath)
		if err != nil {
			log.Log.Error("Read Plug Network Cfg Failed", err)
			return false
		}
		if len(data) < 2 {
			log.Log.Error("Empty Plug Network Cfg")
			return false
		}
		if err = json.Unmarshal(data, cfg); err != nil {
			log.Log.Error("Decode Plug Network failed", err)
			return false
		}
		_, subnet, err := net.ParseCIDR(cfg.Network)
		if err != nil {
			log.Log.Error("Invalid Network", err)
			return false
		}
		s.Ipv4Record = ipam.New(subnet)
		return s.store()
	}
	if err = json.Unmarshal(data, s); err != nil {
		log.Log.Error("Decode Config Failed:", err, "json:", data)
		return false
	}
	return true
}
func (s *PlugStorage) store() bool {
	data, err := json.Marshal(s)
	if err != nil {
		log.Log.Error("Encode failed:", err)
		return false
	}
	if err = os.WriteFile(StoragePath, data, 0644); err != nil {
		log.Log.Error("Write Config Failed: ", err, "\ndata:", data)
		return false
	}
	return true
}
func (s *PlugStorage) AtomicDo(inner func() error) error {
	s.Lock()
	ok := s.load()
	if !ok {
		s.Unlock()
		log.Log.Fatal("load failed")
	}
	err := inner()
	ok = s.store()
	if !ok {
		s.Unlock()
		log.Log.Fatal("store failed")
	}
	s.Unlock()
	return err
}
