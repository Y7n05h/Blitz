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
	FilePerm           = 0644
)

type PlugStorage struct {
	Ipv4Record *ipam.Record
	Mtx        *filemutex.FileMutex `json:"-"`
}
type Cfg struct {
	types.NetConf
}
type PlugNetworkCfg struct {
	ClusterCIDR types.IPNet
	NodeCIDR    types.IPNet
}

func loadPlugNetworkCfg() *PlugNetworkCfg {
	cfg := &PlugNetworkCfg{}
	//json.Unmarshal()
	data, err := os.ReadFile(PlugNetworkCfgPath)
	if err != nil {
		log.Log.Error("Read Plug Network Cfg Failed", err)
		return nil
	}
	if len(data) < 2 {
		log.Log.Error("Empty Plug Network Cfg")
		return nil
	}
	if err = json.Unmarshal(data, cfg); err != nil {
		log.Log.Error("Decode Plug Network failed", err)
		return nil
	}
	return cfg
}
func (p *PlugNetworkCfg) StoreNetworkCfg() error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	err = os.WriteFile(PlugNetworkCfgPath, data, FilePerm)
	if err != nil {
		return err
	}
	return nil
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
	storage.lock()
	ok := storage.load()
	if !ok {
		storage.unlock()
		log.Log.Fatal("load failed")
	}
	storage.unlock()
	return storage, nil
}
func (s *PlugStorage) lock() {
	err := s.Mtx.Lock()
	if err != nil {
		log.Log.Fatal("FileMutex lock Failed:", err)
	}
}
func (s *PlugStorage) unlock() {
	err := s.Mtx.Unlock()
	if err != nil {
		log.Log.Fatal("FileMutex lock Failed:", err)
	}
}
func (s *PlugStorage) load() bool {
	data, err := os.ReadFile(StoragePath)
	if err != nil {
		s.unlock()
		log.Log.Fatal("Read Config Failed", err)
	}
	if len(data) < 2 {
		log.Log.Warn("Empty Config: May be first run this plug in this node?")
		//s.Ipv4Record = s.
		plugNetworkCfg := loadPlugNetworkCfg()
		if plugNetworkCfg == nil {
			return false
		}
		subnet := (*net.IPNet)(&plugNetworkCfg.NodeCIDR)
		s.Ipv4Record = ipam.New(subnet)
		log.Log.Debug("New Ipv4Record: ", s)
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
	log.Log.Debug("Encode data:", data)
	if err = os.WriteFile(StoragePath, data, 0644); err != nil {
		log.Log.Error("Write Config Failed: ", err, "\ndata:", data)
		return false
	}
	return true
}
func (s *PlugStorage) AtomicDo(inner func() error) error {
	s.lock()
	ok := s.load()
	if !ok {
		s.unlock()
		log.Log.Fatal("load failed")
	}
	err := inner()
	ok = s.store()
	if !ok {
		s.unlock()
		log.Log.Fatal("store failed")
	}
	s.unlock()
	return err
}
