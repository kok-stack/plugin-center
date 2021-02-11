#!/bin/bash
DOMAIN=$1
apt update && apt install -y wget
mkdir -p /etc/kubernetes/
mkdir -p /etc/docker/
#下载
wget -O /etc/kubernetes/ca.pem "${DOMAIN}"/meta/{{.ObjectMeta.Namespace}}/{{.ObjectMeta.Name}}/ca/ca.pem
wget -O /etc/kubernetes/node.config "${DOMAIN}"/meta/{{.ObjectMeta.Namespace}}/{{.ObjectMeta.Name}}/nodeconfig/node.config
wget -O kubernetes-server-linux-amd64.tar.gz 'https://dl.k8s.io/v{{.Spec.ClusterVersion}}/kubernetes-server-linux-amd64.tar.gz' && tar -zxvf kubernetes-server-linux-amd64.tar.gz
cp kubernetes/server/bin/kubelet /usr/bin/kubelet
cp kubernetes/server/bin/kube-proxy /usr/bin/kube-proxy
cp kubernetes/server/bin/kubectl /usr/bin/kubectl
chmod a+x /usr/bin/kubelet && chmod a+x /usr/bin/kube-proxy && chmod a+x /usr/bin/kubectl
wget -O /etc/kubernetes/kubelet-config.yaml "${DOMAIN}"/download/{{.ObjectMeta.Namespace}}/{{.ObjectMeta.Name}}/kubelet/kubelet-config.yaml
wget -O /lib/systemd/system/kubelet.service "${DOMAIN}"/download/{{.ObjectMeta.Namespace}}/{{.ObjectMeta.Name}}/kubelet/kubelet.service
wget -O /etc/kubernetes/kubeproxy-config.yaml "${DOMAIN}"/download/{{.ObjectMeta.Namespace}}/{{.ObjectMeta.Name}}/kube-proxy/kubeproxy-config.yaml
wget -O /lib/systemd/system/kubeproxy.service "${DOMAIN}"/download/{{.ObjectMeta.Namespace}}/{{.ObjectMeta.Name}}/kube-proxy/kubeproxy.service
wget -O /etc/docker/daemon.json "${DOMAIN}"/download/{{.ObjectMeta.Namespace}}/{{.ObjectMeta.Name}}/docker/daemon.json

#写hosts
echo '{{.Spec.AccessSpec.Address}} {{.Status.ApiServer.SvcName}}.{{.ObjectMeta.Namespace}}' >>/etc/hosts
#安装docker
curl -fsSL https://get.docker.com | bash -s docker --mirror Aliyun
service docker restart
#启动kubelet,kube-proxy
service kubelet start
service kubeproxy start

# curl -fsSL http://localhost:7788/download/test/test/node/install.sh | bash -s http://localhost:7788
