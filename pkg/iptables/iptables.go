package iptables

import (
	"blitz/pkg/ipnet"
	"blitz/pkg/log"

	"github.com/coreos/go-iptables/iptables"
)

const (
	IPv4 = iptables.ProtocolIPv4
	IPv6 = iptables.ProtocolIPv6
)

type Rule struct {
	table    string
	pos      int
	chain    string
	ruleSpec []string
}

func CreateChain(table, chain string, protocol iptables.Protocol) {
	ipt, err := iptables.NewWithProtocol(protocol)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Log.Errorf("Failed to setup IPTables. iptables binary was not found: %v", err)
		return
	}
	err = ipt.ClearChain(table, chain)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Log.Errorf("Failed to setup IPTables. Error on creating the chain: %v", err)
		return
	}
}

func MasqRules(clusterCIDR *ipnet.IPNet, podCIDR *ipnet.IPNet, chainName string, protocol iptables.Protocol) []Rule {
	n := clusterCIDR.String()
	sn := podCIDR.String()
	supportsRandomFully := false
	ipt, err := iptables.New()
	if err == nil {
		supportsRandomFully = ipt.HasRandomFully()
	}
	var multicast string
	if protocol == IPv4 {
		multicast = "224.0.0.0/4"
	} else {
		multicast = "ff00::/8"
	}
	result := []Rule{
		// This rule ensure that the blitz iptables rules are executed before other rules on the node
		{"nat", 1, "POSTROUTING", []string{"-m", "comment", "--comment", "blitzd masq", "-j", chainName}},
		// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
		{"nat", -1, chainName, []string{"-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
		// NAT if it's not multicast traffic
		{"nat", -1, chainName, []string{"-s", n, "!", "-d", multicast, "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE"}},
		// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
		{"nat", -1, chainName, []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
		// Masquerade anything headed towards blitz from the host
		{"nat", -1, chainName, []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE"}},
	}
	if supportsRandomFully {
		result[2].ruleSpec = append(result[2].ruleSpec, "--random-fully")
		result[4].ruleSpec = append(result[4].ruleSpec, "--random-fully")
	}
	return result
}
func ApplyRulesWithCheck(rules []Rule, protocol iptables.Protocol) error {
	ipt, err := iptables.NewWithProtocol(protocol)
	if err != nil {
		log.Log.Errorf("Setup IPtables Failed:%v", err)
		return err
	}
	for _, rule := range rules {
		if rule.pos >= 0 {
			exist, err := ipt.Exists(rule.table, rule.chain, rule.ruleSpec...)
			if err != nil {
				log.Log.Errorf("Check Iptables chain %s Failed:%v", rule.chain, err)
				return err
			}
			if exist {
				continue
			}
			err = ipt.Insert(rule.table, rule.chain, rule.pos, rule.ruleSpec...)
			if err != nil {
				log.Log.Errorf("Insert IPtables Failed:%v", err)
				return err
			}
		} else {
			err := ipt.AppendUnique(rule.table, rule.chain, rule.ruleSpec...)
			if err != nil {
				log.Log.Errorf("Append IPtables Failed:%v", err)
				return err
			}
		}
	}
	return nil
}
