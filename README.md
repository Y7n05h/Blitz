## Blitz 

Blitz 是一个快速部署的 CNI 网络插件。
Blitz 是一个追求带给闪电战般用户体验的 CNI 网络插件。低延迟和高吞吐量是 Blitz 永恒的追求。

### 部署

Blitz 十分易于部署，通常来说您不必修改配置文件，Blitz 将从您的 Kubernetes 集群中自动获取集群的相关网络配置，并自动为其生成必要的相关配置文件。
您仅仅需要在部署完成 Kubernetes 后通过 kubectl 执行一下命令：

```bash
kubectl apply -f https://raw.githubusercontent.com/Y7n05h/Blitz/master/doc/blitz.yaml
```

### LICENSE

本项目所有代码使用 Apache 2.0 进行许可。
