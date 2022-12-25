package config

import (
	"blitz/pkg/ipam"
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	"encoding/json"
	"errors"
	"os"
	"path"

	"github.com/alexflint/go-filemutex"
	"github.com/containernetworking/cni/pkg/types"
)

const (
	StorageDir      = "/run/blitz/"
	StorageFileName = "config.json"
	StoragePath     = StorageDir + StorageFileName
	FilePerm        = 0644
)

type PlugStorage struct {
	Ipv4Record *ipam.Ipam
	Mtx        *filemutex.FileMutex `json:"-"`
	Ipv4Cfg    *NetworkCfg
}
type CniRuntimeCfg struct {
	types.NetConf
}
type NetworkCfg struct {
	//All filed in NetworkCfg is Read-only
	ClusterCIDR ipnet.IPNet
	PodCIDR     ipnet.IPNet
}

func LoadCfg(data []byte) (*CniRuntimeCfg, error) {
	cfg := &CniRuntimeCfg{}
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
func CreateStorage(cfg NetworkCfg) (*PlugStorage, error) {
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
	storage := &PlugStorage{Mtx: mtx, Ipv4Cfg: &cfg, Ipv4Record: ipam.New(&cfg.PodCIDR)}

	//无需加锁，此时不存在并发操作

	ok := storage.store()
	if !ok {
		log.Log.Fatal("load failed")
	}
	return storage, nil
}
func LoadStorage() (*PlugStorage, error) {
	if _, err := os.Stat(StoragePath); err != nil {
		log.Log.Errorf("%#v", err)
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
	log.Log.Debugf("[LOCK]")
	err := s.Mtx.Lock()
	if err != nil {
		log.Log.Fatal("FileMutex lock Failed:", err)
	}
}
func (s *PlugStorage) unlock() {
	log.Log.Debugf("[UNLOCK]")
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
		log.Log.Fatal("Load Empty Storage!")
	}
	if err = json.Unmarshal(data, s); err != nil {
		log.Log.Error("Decode Config Failed:", err, "json:", data)
		return false
	}
	log.Log.Debugf("Load Plug Storage Success: %s", data)
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
	log.Log.Debugf("Store Plug Storage Success: %s", data)
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
