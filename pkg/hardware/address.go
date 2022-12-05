package hardware

import (
	"crypto/rand"
	"encoding/json"
	"net"
	"tiny_cni/internal/log"
)

type Address net.HardwareAddr

var _ json.Unmarshaler = (*Address)(nil)
var _ json.Marshaler = (*Address)(nil)

func GenHardwareAddr() Address {
	mac := make([]byte, 6)
	_, err := rand.Read(mac)
	if err != nil {
		log.Log.Fatal("Gen Mac Addr Failed")
	}
	mac[0] &= 0xfe
	mac[0] |= 0x2
	return mac
}
func (r Address) MarshalJSON() ([]byte, error) {
	ptr := (*net.HardwareAddr)(&r)
	return json.Marshal(ptr.String())
}
func (r *Address) UnmarshalJSON(data []byte) error {
	var macString string
	err := json.Unmarshal(data, &macString)
	if err != nil {
		return nil
	}
	ptr := (*net.HardwareAddr)(r)
	mac, err := net.ParseMAC(macString)
	if err != nil {
		return err
	}
	*ptr = mac
	return nil
}
func (r *Address) ToNetHardwareAddr() net.HardwareAddr {
	return *(*net.HardwareAddr)(r)
}
