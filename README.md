## Blitz 

Blitz 是一个快速部署的 CNI 网络插件。
Blitz 更够以 VXLAN 模式或 Host-gw 模式完成组网，Blitz 也实现了 IPv4 和 IPv6 双栈解析。
Blitz 是一个追求带给闪电战般用户体验的 CNI 网络插件。低延迟和高吞吐量是 Blitz 永恒的追求。


### 部署

Blitz 十分易于部署，通常来说您不必修改配置文件，Blitz 将从您的 Kubernetes 集群中自动获取集群的相关网络配置，并自动为其生成必要的相关配置文件。
您仅仅需要在部署完成 Kubernetes 后通过 kubectl 执行一下命令：

```bash
kubectl apply -f https://raw.githubusercontent.com/Y7n05h/Blitz/master/doc/blitz.yaml
```

### 配置

对 Blitz 的配置需要通过修改 Blitzd 的命令行参数来完成。
Blitzd 接受下列命令行参数：
--version[=bool|true] 
查看 Blitzd 的版本和构建信息
--ip-Masq[=bool|true] 
启用 IP Masq.
--ClusterCIDR=string
配置集群的 CIDR，接受以 comma 分割的 CIDR，此处的配置应当与 api server 的 --service-cluster-ip-range 参数保持一致。
--mode=string
选择使用 vxlan 模式或 host-gw 模式，如未提供 mode 参数则默认使用 vxlan 模式。

### RoadMap
- [x] 实现 Blitz 的 VXLAN 模式和 host-gw 模式
- [x] 实现 ip-masq
- [ ] 适配 [KEP-2593: Enhanced NodeIPAM to support Discontiguous Cluster CIDR](https://github.com/kubernetes/enhancements/tree/master/keps/sig-network/2593-multiple-cluster-cidrs)
- [ ] 通过 BGP 实现更复杂的网络结构（目前 Blitz 要求所有 Node 均满足 2层可达）
- [ ] 通过 eBPF 提高性能
- [ ] 通过 eBPF 提高可观测性
- [ ] ...


### LICENSE

本项目所有代码使用 Apache 2.0 进行许可。
