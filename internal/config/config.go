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
	StorageDir      = "/run/tcni/"
	StorageFileName = "config.json"
	StorageFilePath = StorageDir + StorageFileName
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
	if _, err := os.Stat(StorageFilePath); err == nil {
		if err = os.MkdirAll(StorageDir, 0750); err != nil {
			return nil, err
		}
		if file, err := os.Create(StorageFilePath); err != nil {
			log.Log.Fatal("Init config failed:", err)
		} else {
			if err = file.Close(); err != nil {
				log.Log.Fatal("Close config failed:", err)
			}
		}
	}
	mtx, err := newFileMutex(StorageDir)
	if err != nil {
		return nil, err
	}
	storage := &PlugStorage{Mtx: mtx}
	storage.Load()
	return storage, nil
}
func (s *PlugStorage) Lock() {
	err := s.Mtx.Lock()
	if err != nil {
		log.Log.Fatalf("FileMutex Lock Failed")
	}
}
func (s *PlugStorage) Unlock() {
	err := s.Mtx.Unlock()
	if err != nil {
		log.Log.Fatalf("FileMutex Lock Failed")
	}
}
func (s *PlugStorage) Load() {
	s.Lock()
	defer s.Unlock()
	data, err := os.ReadFile(StorageFilePath)
	if err != nil {
		log.Log.Fatalf("Read Config Failed")
	}
	if err = json.Unmarshal(data, s); err != nil {
		log.Log.Fatalf("Encode Config Failed")
	}
}
func (s *PlugStorage) Store() {
	s.Lock()
	defer s.Unlock()
	data, err := json.Marshal(s)
	s.Ipv4Record = nil
	s.Mtx = nil
	if err != nil {
		log.Log.Error("Encode failed:", err)
	} else {
		if err = os.WriteFile(StorageFilePath, data, 0644); err != nil {
			log.Log.Fatalf("Write Config Failed")
		}
	}
}
