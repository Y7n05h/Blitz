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

func MasqRules(clusterCIDR *ipnet.IPNet, podCIDR *ipnet.IPNet, chainName string) []Rule {
	n := clusterCIDR.String()
	sn := podCIDR.String()
	supportsRandomFully := false
	ipt, err := iptables.New()
	if err == nil {
		supportsRandomFully = ipt.HasRandomFully()
	}

	if supportsRandomFully {
		return []Rule{
			// This rule ensure that the blitz iptables rules are executed before other rules on the node
			{"nat", 1, "POSTROUTING", []string{"-m", "comment", "--comment", "blitzd masq", "-j", chainName}},
			// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
			{"nat", -1, chainName, []string{"-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
			// NAT if it's not multicast traffic
			{"nat", -1, chainName, []string{"-s", n, "!", "-d", "224.0.0.0/4", "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE", "--random-fully"}},
			// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
			{"nat", -1, chainName, []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
			// Masquerade anything headed towards blitz from the host
			{"nat", -1, chainName, []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE", "--random-fully"}},
		}
	} else {
		return []Rule{
			// This rule ensure that the blitz iptables rules are executed before other rules on the node
			{"nat", 1, "POSTROUTING", []string{"-m", "comment", "--comment", "blitzd masq", "-j", chainName}},
			// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
			{"nat", -1, chainName, []string{"-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
			// NAT if it's not multicast traffic
			{"nat", -1, chainName, []string{"-s", n, "!", "-d", "224.0.0.0/4", "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE"}},
			// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
			{"nat", -1, chainName, []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
			// Masquerade anything headed towards blitz from the host
			{"nat", -1, chainName, []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE"}},
		}
	}
}

func MasqIP6Rules(ipn *ipnet.IPNet, subnet *ipnet.IPNet, chainName string) []Rule {
	n := ipn.String()
	sn := subnet.String()
	supportsRandomFully := false
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err == nil {
		supportsRandomFully = ipt.HasRandomFully()
	}

	if supportsRandomFully {
		return []Rule{
			// This rule ensure that the blitz iptables rules are executed before other rules on the node
			{"nat", 1, "POSTROUTING", []string{"-m", "comment", "--comment", "blitzd masq", "-j", chainName}},
			// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
			{"nat", -1, chainName, []string{"-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
			// NAT if it's not multicast traffic
			{"nat", -1, chainName, []string{"-s", n, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE", "--random-fully"}},
			// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
			{"nat", -1, chainName, []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
			// Masquerade anything headed towards blitz from the host
			{"nat", -1, chainName, []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE", "--random-fully"}},
		}
	} else {
		return []Rule{
			// This rule ensure that the blitz iptables rules are executed before other rules on the node
			{"nat", 1, "POSTROUTING", []string{"-m", "comment", "--comment", "blitzd masq", "-j", chainName}},
			// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0)
			{"nat", -1, chainName, []string{"-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
			// NAT if it's not multicast traffic
			{"nat", -1, chainName, []string{"-s", n, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE"}},
			// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
			{"nat", -1, chainName, []string{"!", "-s", n, "-d", sn, "-m", "comment", "--comment", "blitzd masq", "-j", "RETURN"}},
			// Masquerade anything headed towards blitz from the host
			{"nat", -1, chainName, []string{"!", "-s", n, "-d", n, "-m", "comment", "--comment", "blitzd masq", "-j", "MASQUERADE"}},
		}
	}
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
