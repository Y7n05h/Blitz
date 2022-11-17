package config

import (
	"encoding/json"
	"os"
	"path"
	"tiny_cni/internal/log"
	"tiny_cni/pkg/ipam"

	"github.com/alexflint/go-filemutex"
	"github.com/containernetworking/cni/pkg/types"
)

const (
	StoragePath     = "/run/tiny_cni/"
	StorageFileName = "config.json"
	StorageFilePath = StoragePath + StorageFileName
)

type PlugStorage struct {
	Ipv4Record *ipam.Record         `json:"ipv4"`
	Mtx        *filemutex.FileMutex `json:"-"`
}
type Cfg struct {
	types.NetConf
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
	if err := os.MkdirAll(StoragePath, 0750); err != nil {
		return nil, err
	}
	mtx, err := newFileMutex(StorageFilePath)
	storage := &PlugStorage{Mtx: mtx}
	if mtx.Lock() != err {
		log.Log.Fatalf("Lock failed")
	}
	data, err := os.ReadFile(StorageFilePath)
	if err != nil {
		return storage, nil
	}
	if err = json.Unmarshal(data, storage); err != nil {
		log.Log.Error("Unmarshal failed")
		err2 := mtx.Unlock()
		if err2 != nil {
			log.Log.Error("Unlock failed")
		}
		return nil, err
	}
	return storage, nil
}
func (s *PlugStorage) Store() {
	data, err := json.Marshal(s)
	s.Ipv4Record = nil
	s.Mtx = nil
	if err != nil {
		log.Log.Error("Encode failed:", err)
	} else {
		if err = os.WriteFile(StorageFilePath, data, 0644); err != nil {
			log.Log.Error("Store failed:", err)
		}
	}
	if err = s.Mtx.Unlock(); err != nil {
		log.Log.Error("Unlock failed: ", err)
		return
	}
}
