#!/bin/bash
service kubelet stop
service kubeproxy stop
service docker stop

rm -rf /etc/kubernetes
rm -f /lib/systemd/system/kubelet.service
rm -f /lib/systemd/system/kubeproxy.service

# curl -fsSL http://localhost:7788/download/test/test/node/uninstall.sh | bash
